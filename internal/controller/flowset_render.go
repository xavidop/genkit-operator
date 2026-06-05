/*
Copyright 2026 Xavier Portilla Edo.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"encoding/json"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// flowSetMountRoot is the path under which all flow content is mounted in
// the FlowSet runtime Pod. See docs/runtime-contract.md (multi-flow).
const flowSetMountRoot = "/genkit"

// flowSetManifest is the top-level descriptor written to
// /genkit/manifest.json. The runner reads it to discover every flow served
// by the Pod, its entrypoint prompt, and where its credentials live.
type flowSetManifest struct {
	Flows []flowManifestEntry `json:"flows"`
}

type flowManifestEntry struct {
	// Name is the flow name and HTTP route segment (POST /<name>).
	Name string `json:"name"`
	// Entrypoint is the prompt name invoked when POST /<name> is called.
	Entrypoint string `json:"entrypoint"`
	// Dir is the absolute path of this flow's content directory inside
	// the Pod. The runner expects {prompts,tools,config.json,credentials}
	// directly underneath.
	Dir string `json:"dir"`
}

// resolvedFlowSetFlow holds the dependency objects for a single flow
// inside a FlowSet, already fetched from the API server.
type resolvedFlowSetFlow struct {
	template genkitv1alpha1.FlowSetFlow
	prompts  []genkitv1alpha1.Prompt
	tools    []genkitv1alpha1.Tool
	model    genkitv1alpha1.Model
	plugin   genkitv1alpha1.PluginConfig
	// secretName is the name of the Secret referenced by plugin.spec.
	// credentialsRef. Used to mount per-flow credentials at
	// /genkit/flows/<flow>/credentials/.
	secretName string
	// secret is the resolved credentials Secret. Its data is included in
	// the FlowSet contentHash so that rotating credentials triggers a
	// Deployment rollout — the runner re-reads credential files on start.
	secret *corev1.Secret
}

// resolvedFlowSet aggregates resolved per-flow dependencies.
type resolvedFlowSet struct {
	flows []resolvedFlowSetFlow
}

// renderedFlowSet is the bundle of objects produced by renderFlowSet.
type renderedFlowSet struct {
	// manifestCM holds /genkit/manifest.json describing every flow.
	manifestCM *corev1.ConfigMap
	// perFlowCMs are the three ConfigMaps per flow (prompts, tools, config).
	perFlowCMs []*corev1.ConfigMap

	service *corev1.Service
	deploy  *appsv1.Deployment

	// contentHash is SHA-256 over the manifest and every per-flow CM data
	// map. It is annotated on the Pod template to drive rollouts.
	contentHash string
}

// flowSetCMName builds a ConfigMap name for a flow inside a FlowSet.
// kind is one of "prompts", "tools", "config".
func flowSetCMName(fs *genkitv1alpha1.FlowSet, flow, kind string) string {
	return fmt.Sprintf("%s-%s-%s", fs.Name, flow, kind)
}

func flowSetManifestCMName(fs *genkitv1alpha1.FlowSet) string {
	return fmt.Sprintf("%s-manifest", fs.Name)
}

// flowSetFlowDir returns the in-Pod content directory for a single flow.
func flowSetFlowDir(name string) string {
	return fmt.Sprintf("%s/flows/%s", flowSetMountRoot, name)
}

// renderFlowSet produces the full set of child objects for a FlowSet.
// It does not touch the API server.
func renderFlowSet(fs *genkitv1alpha1.FlowSet, deps *resolvedFlowSet) (*renderedFlowSet, error) {
	baseLabels := managedLabels(map[string]string{
		genkitv1alpha1.LabelFlowSet: fs.Name,
	})

	// Build the manifest.
	manifest := flowSetManifest{Flows: make([]flowManifestEntry, 0, len(deps.flows))}
	allCMData := []map[string]string{}

	// Per-flow ConfigMaps.
	var perFlowCMs []*corev1.ConfigMap
	for _, rf := range deps.flows {
		// Prompts CM: <prompt-name>.prompt -> raw dotprompt content.
		promptsData := map[string]string{}
		for _, p := range rf.prompts {
			promptsData[p.Name+".prompt"] = p.Spec.Content
		}
		pCM := newConfigMap(
			flowSetCMName(fs, rf.template.Name, "prompts"),
			fs.Namespace, baseLabels, promptsData,
		)
		perFlowCMs = append(perFlowCMs, pCM)
		allCMData = append(allCMData, promptsData)

		// Tools CM.
		toolsData := map[string]string{}
		for _, t := range rf.tools {
			payload, err := renderToolDescriptor(&t)
			if err != nil {
				return nil, fmt.Errorf("flow %q tool %q: %w", rf.template.Name, t.Name, err)
			}
			toolsData[t.Name+".json"] = string(payload)
		}
		tCM := newConfigMap(
			flowSetCMName(fs, rf.template.Name, "tools"),
			fs.Namespace, baseLabels, toolsData,
		)
		perFlowCMs = append(perFlowCMs, tCM)
		allCMData = append(allCMData, toolsData)

		// Config CM: config.json with default model + plugin + credentialsDir.
		cfgBytes, err := renderFlowSetFlowConfig(&rf)
		if err != nil {
			return nil, fmt.Errorf("flow %q render config: %w", rf.template.Name, err)
		}
		cfgData := map[string]string{"config.json": string(cfgBytes)}
		cCM := newConfigMap(
			flowSetCMName(fs, rf.template.Name, "config"),
			fs.Namespace, baseLabels, cfgData,
		)
		perFlowCMs = append(perFlowCMs, cCM)
		allCMData = append(allCMData, cfgData)

		// Manifest entry.
		entrypoint := ""
		if len(rf.template.Prompts) > 0 {
			entrypoint = rf.template.Prompts[0].Name
		}
		manifest.Flows = append(manifest.Flows, flowManifestEntry{
			Name:       rf.template.Name,
			Entrypoint: entrypoint,
			Dir:        flowSetFlowDir(rf.template.Name),
		})
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	manifestData := map[string]string{"manifest.json": string(manifestBytes)}
	manifestCM := newConfigMap(
		flowSetManifestCMName(fs), fs.Namespace, baseLabels, manifestData,
	)
	allCMData = append(allCMData, manifestData)

	// Fold the data of every distinct credentials Secret into the hash so
	// credential rotation triggers a rollout.
	seenSecret := map[string]struct{}{}
	for _, rf := range deps.flows {
		if rf.secret == nil {
			continue
		}
		if _, dup := seenSecret[rf.secret.Name]; dup {
			continue
		}
		seenSecret[rf.secret.Name] = struct{}{}
		if payload := secretHashPayload(rf.secret); payload != nil {
			allCMData = append(allCMData, payload)
		}
	}

	hash := computeContentHash(allCMData...)

	svc := newFlowSetService(fs, baseLabels)
	dep := newFlowSetDeployment(fs, deps, baseLabels, hash)

	return &renderedFlowSet{
		manifestCM:  manifestCM,
		perFlowCMs:  perFlowCMs,
		service:     svc,
		deploy:      dep,
		contentHash: hash,
	}, nil
}

// renderFlowSetFlowConfig produces the per-flow config.json. It mirrors
// the single-Flow runtimeConfig and adds credentialsDir + credentialKeys
// so the runner can load API keys from files instead of environment
// variables.
func renderFlowSetFlowConfig(rf *resolvedFlowSetFlow) ([]byte, error) {
	type flowConfig struct {
		DefaultModel   *defaultModel  `json:"defaultModel,omitempty"`
		Plugin         *pluginPayload `json:"plugin,omitempty"`
		CredentialsDir string         `json:"credentialsDir,omitempty"`
		CredentialKeys []string       `json:"credentialKeys,omitempty"`
	}
	cfg := flowConfig{}
	dm := &defaultModel{
		Provider: rf.model.Spec.Provider,
		Model:    rf.model.Spec.Model,
	}
	if rf.model.Spec.Info != nil {
		dm.Info = rf.model.Spec.Info.Raw
	}
	if rf.model.Spec.DefaultConfig != nil {
		dm.Config = rf.model.Spec.DefaultConfig.Raw
	}
	cfg.DefaultModel = dm
	pp := &pluginPayload{
		Type:   rf.plugin.Spec.Type,
		Region: rf.plugin.Spec.Region,
	}
	if rf.plugin.Spec.ExtraConfig != nil {
		pp.ExtraConfig = rf.plugin.Spec.ExtraConfig.Raw
	}
	cfg.Plugin = pp
	cfg.CredentialsDir = fmt.Sprintf("%s/credentials", flowSetFlowDir(rf.template.Name))
	cfg.CredentialKeys = rf.plugin.Spec.CredentialKeys
	return json.MarshalIndent(cfg, "", "  ")
}

func newFlowSetService(fs *genkitv1alpha1.FlowSet, labels map[string]string) *corev1.Service {
	port := int32(8080)
	if fs.Spec.Port != nil {
		port = *fs.Spec.Port
	}
	svcType := corev1.ServiceTypeClusterIP
	if fs.Spec.ServiceType != "" {
		svcType = corev1.ServiceType(fs.Spec.ServiceType)
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name,
			Namespace: fs.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       port,
				TargetPort: intstr.FromInt32(port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func newFlowSetDeployment(fs *genkitv1alpha1.FlowSet, deps *resolvedFlowSet, labels map[string]string, contentHash string) *appsv1.Deployment {
	port := int32(8080)
	if fs.Spec.Port != nil {
		port = *fs.Spec.Port
	}
	replicas := int32(1)
	if fs.Spec.Replicas != nil {
		replicas = *fs.Spec.Replicas
	}
	pullPolicy := corev1.PullIfNotPresent
	if fs.Spec.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(fs.Spec.ImagePullPolicy)
	}

	// Build volumes + mounts.
	// Per flow we add: 3 ConfigMap volumes (prompts/tools/config) plus at
	// most 1 Secret volume; the manifest contributes the +1.
	volumes := make([]corev1.Volume, 0, 1+4*len(deps.flows))
	mounts := make([]corev1.VolumeMount, 0, 1+4*len(deps.flows))

	// Manifest mount (one file).
	volumes = append(volumes, corev1.Volume{
		Name: "genkit-manifest",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: flowSetManifestCMName(fs),
				},
			},
		},
	})
	mounts = append(mounts, corev1.VolumeMount{
		Name:      "genkit-manifest",
		MountPath: flowSetMountRoot + "/manifest.json",
		SubPath:   "manifest.json",
		ReadOnly:  true,
	})

	// One volume + mount per (flow, kind). Secret volumes are deduplicated
	// across flows by secret name; the mount path is per-flow.
	secretVolumeAdded := map[string]string{} // secretName -> volume name
	for _, rf := range deps.flows {
		flow := rf.template.Name
		// Prompts dir volume.
		pVolName := "p-" + flow
		volumes = append(volumes, corev1.Volume{
			Name: pVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: flowSetCMName(fs, flow, "prompts"),
					},
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      pVolName,
			MountPath: flowSetFlowDir(flow) + "/prompts",
			ReadOnly:  true,
		})

		// Tools dir volume.
		tVolName := "t-" + flow
		volumes = append(volumes, corev1.Volume{
			Name: tVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: flowSetCMName(fs, flow, "tools"),
					},
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      tVolName,
			MountPath: flowSetFlowDir(flow) + "/tools",
			ReadOnly:  true,
		})

		// Config file volume (single key, mount via subPath).
		cVolName := "c-" + flow
		volumes = append(volumes, corev1.Volume{
			Name: cVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: flowSetCMName(fs, flow, "config"),
					},
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      cVolName,
			MountPath: flowSetFlowDir(flow) + "/config.json",
			SubPath:   "config.json",
			ReadOnly:  true,
		})

		// Credentials volume (Secret); dedup by secret name.
		secretVol, ok := secretVolumeAdded[rf.secretName]
		if !ok {
			secretVol = "s-" + sha256ShortHex(rf.secretName)
			secretVolumeAdded[rf.secretName] = secretVol
			volumes = append(volumes, corev1.Volume{
				Name: secretVol,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: rf.secretName,
					},
				},
			})
		}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      secretVol,
			MountPath: flowSetFlowDir(flow) + "/credentials",
			ReadOnly:  true,
		})
	}

	container := corev1.Container{
		Name:            "flowset",
		Image:           fs.Spec.Image,
		ImagePullPolicy: pullPolicy,
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		}},
		Env:          fs.Spec.Env,
		VolumeMounts: mounts,
		Resources:    fs.Spec.Resources,
	}

	podLabels := managedLabels(map[string]string{
		genkitv1alpha1.LabelFlowSet: fs.Name,
	})
	annotations := map[string]string{
		genkitv1alpha1.AnnotationContentHash: contentHash,
	}

	// Sort volumes deterministically so SSA doesn't churn.
	sort.SliceStable(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })
	sort.SliceStable(container.VolumeMounts, func(i, j int) bool {
		return container.VolumeMounts[i].MountPath < container.VolumeMounts[j].MountPath
	})

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name,
			Namespace: fs.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes:    volumes,
				},
			},
		},
	}
}

// sha256ShortHex returns the first 10 hex chars of sha256(s). Used to
// build deterministic, DNS-1123 volume names from arbitrary secret names.
func sha256ShortHex(s string) string {
	return sha256Hex([]byte(s))[:10]
}
