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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// flowMountRoot is the path under which the operator mounts the runtime
// contract files described in docs/runtime-contract.md.
const flowMountRoot = "/genkit"

// runtimeConfig is the JSON object the operator writes to
// /genkit/config.json. It uses genkit-go's standard JSON field names so
// any Genkit runtime (Go, Node, Java, Python) can decode it.
type runtimeConfig struct {
	DefaultModel *defaultModel  `json:"defaultModel,omitempty"`
	Plugin       *pluginPayload `json:"plugin,omitempty"`
}

type defaultModel struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Info     json.RawMessage `json:"info,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
}

type pluginPayload struct {
	Type        string          `json:"type"`
	Region      string          `json:"region,omitempty"`
	ExtraConfig json.RawMessage `json:"extraConfig,omitempty"`
}

// renderedFlow is the bundle of objects produced by renderFlow for a
// resolved Flow + its dependencies. The controller applies each in turn
// and stamps the content hash onto the Deployment pod template.
type renderedFlow struct {
	promptsCM *corev1.ConfigMap
	toolsCM   *corev1.ConfigMap
	configCM  *corev1.ConfigMap
	service   *corev1.Service
	deploy    *appsv1.Deployment

	// contentHash is the SHA-256 over the rendered ConfigMap payloads. It
	// is set on the Deployment pod template annotation so any change to
	// prompts, tools, or config triggers a rollout.
	contentHash string
}

// resolvedDeps holds the dependency objects already fetched from the API
// server, in the order needed to render the Flow.
type resolvedDeps struct {
	prompts []genkitv1alpha1.Prompt
	tools   []genkitv1alpha1.Tool
	model   *genkitv1alpha1.Model
	plugin  *genkitv1alpha1.PluginConfig
}

func cmName(flow *genkitv1alpha1.Flow, suffix string) string {
	return fmt.Sprintf("%s-%s", flow.Name, suffix)
}

// renderFlow produces the full set of child objects for a Flow given its
// resolved dependencies. It does not touch the API server.
func renderFlow(flow *genkitv1alpha1.Flow, deps *resolvedDeps) (*renderedFlow, error) {
	baseLabels := managedLabels(map[string]string{
		genkitv1alpha1.LabelFlow: flow.Name,
	})

	// Prompts ConfigMap: <name>.prompt -> raw dotprompt content.
	promptsData := map[string]string{}
	for _, p := range deps.prompts {
		promptsData[p.Name+".prompt"] = p.Spec.Content
	}
	promptsCM := newConfigMap(cmName(flow, "prompts"), flow.Namespace, baseLabels, promptsData)

	// Tools ConfigMap: <name>.json -> serialized ToolDefinition+implementation.
	toolsData := map[string]string{}
	for _, t := range deps.tools {
		payload, err := renderToolDescriptor(&t)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", t.Name, err)
		}
		toolsData[t.Name+".json"] = string(payload)
	}
	toolsCM := newConfigMap(cmName(flow, "tools"), flow.Namespace, baseLabels, toolsData)

	// Config ConfigMap: config.json.
	configBytes, err := renderRuntimeConfig(deps)
	if err != nil {
		return nil, fmt.Errorf("render runtime config: %w", err)
	}
	configCM := newConfigMap(cmName(flow, "config"), flow.Namespace, baseLabels, map[string]string{
		"config.json": string(configBytes),
	})

	// Hash all three rendered payloads in a deterministic order.
	hash := computeContentHash(promptsCM.Data, toolsCM.Data, configCM.Data)

	service := newService(flow, baseLabels)
	deploy := newDeployment(flow, deps, baseLabels, hash)

	return &renderedFlow{
		promptsCM:   promptsCM,
		toolsCM:     toolsCM,
		configCM:    configCM,
		service:     service,
		deploy:      deploy,
		contentHash: hash,
	}, nil
}

func newConfigMap(name, namespace string, labels, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func renderToolDescriptor(t *genkitv1alpha1.Tool) ([]byte, error) {
	var def map[string]any
	if len(t.Spec.Definition.Raw) > 0 {
		if err := json.Unmarshal(t.Spec.Definition.Raw, &def); err != nil {
			return nil, fmt.Errorf("invalid definition JSON: %w", err)
		}
	} else {
		def = map[string]any{}
	}
	impl := map[string]any{}
	if t.Spec.Implementation.FlowRef != nil {
		impl["flowRef"] = map[string]any{"name": t.Spec.Implementation.FlowRef.Name}
	}
	if t.Spec.Implementation.HTTP != nil {
		http := map[string]any{
			"url":    t.Spec.Implementation.HTTP.URL,
			"method": string(t.Spec.Implementation.HTTP.Method),
		}
		if t.Spec.Implementation.HTTP.HeadersSecretRef != nil {
			http["headersSecretRef"] = map[string]any{
				"name": t.Spec.Implementation.HTTP.HeadersSecretRef.Name,
			}
		}
		impl["http"] = http
	}
	def["implementation"] = impl
	return json.Marshal(def)
}

func renderRuntimeConfig(deps *resolvedDeps) ([]byte, error) {
	cfg := runtimeConfig{}
	if deps.model != nil {
		dm := &defaultModel{
			Provider: deps.model.Spec.Provider,
			Model:    deps.model.Spec.Model,
		}
		if deps.model.Spec.Info != nil {
			dm.Info = deps.model.Spec.Info.Raw
		}
		if deps.model.Spec.DefaultConfig != nil {
			dm.Config = deps.model.Spec.DefaultConfig.Raw
		}
		cfg.DefaultModel = dm
	}
	if deps.plugin != nil {
		pp := &pluginPayload{
			Type:   deps.plugin.Spec.Type,
			Region: deps.plugin.Spec.Region,
		}
		if deps.plugin.Spec.ExtraConfig != nil {
			pp.ExtraConfig = deps.plugin.Spec.ExtraConfig.Raw
		}
		cfg.Plugin = pp
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func computeContentHash(payloads ...map[string]string) string {
	type kv struct{ K, V string }
	var flat []kv
	for _, m := range payloads {
		for k, v := range m {
			flat = append(flat, kv{K: k, V: v})
		}
	}
	sort.Slice(flat, func(i, j int) bool { return flat[i].K < flat[j].K })
	buf, _ := json.Marshal(flat)
	return sha256Hex(buf)
}

func newService(flow *genkitv1alpha1.Flow, labels map[string]string) *corev1.Service {
	port := int32(8080)
	if flow.Spec.Port != nil {
		port = *flow.Spec.Port
	}
	svcType := corev1.ServiceTypeClusterIP
	if flow.Spec.ServiceType != "" {
		svcType = corev1.ServiceType(flow.Spec.ServiceType)
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name,
			Namespace: flow.Namespace,
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

func newDeployment(flow *genkitv1alpha1.Flow, deps *resolvedDeps, labels map[string]string, contentHash string) *appsv1.Deployment {
	port := int32(8080)
	if flow.Spec.Port != nil {
		port = *flow.Spec.Port
	}
	replicas := int32(1)
	if flow.Spec.Replicas != nil {
		replicas = *flow.Spec.Replicas
	}
	pullPolicy := corev1.PullIfNotPresent
	if flow.Spec.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(flow.Spec.ImagePullPolicy)
	}

	container := corev1.Container{
		Name:            "flow",
		Image:           flow.Spec.Image,
		ImagePullPolicy: pullPolicy,
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		}},
		Env: flow.Spec.Env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "genkit-prompts", MountPath: flowMountRoot + "/prompts"},
			{Name: "genkit-tools", MountPath: flowMountRoot + "/tools"},
			{Name: "genkit-config", MountPath: flowMountRoot},
		},
		Resources: flow.Spec.Resources,
	}
	if deps.plugin != nil {
		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: deps.plugin.Spec.CredentialsRef.Name,
				},
			},
		})
	}

	podLabels := managedLabels(map[string]string{
		genkitv1alpha1.LabelFlow: flow.Name,
	})
	annotations := map[string]string{
		genkitv1alpha1.AnnotationContentHash: contentHash,
	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name,
			Namespace: flow.Namespace,
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
					Volumes: []corev1.Volume{
						{
							Name: "genkit-prompts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cmName(flow, "prompts"),
									},
								},
							},
						},
						{
							Name: "genkit-tools",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cmName(flow, "tools"),
									},
								},
							},
						},
						{
							Name: "genkit-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cmName(flow, "config"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return dep
}

// _ ensures the resource package stays imported for future scaling fields
// without breaking the build if tests are temporarily removed.
var _ = resource.MustParse
