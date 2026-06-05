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
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// FlowSetReconciler reconciles FlowSet resources. A FlowSet renders to a
// SINGLE Deployment + Service that serves every declared flow on the same
// Pod. Per-flow content (prompts, tools, model, credentials) is mounted
// under /genkit/flows/<name>/ per docs/runtime-contract.md.
type FlowSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=flowsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=flowsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=flowsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=genkit.dev,resources=prompts;tools;models;pluginconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;services;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile resolves every flow's dependencies and reconciles the single
// shared Deployment + Service + per-flow ConfigMaps.
func (r *FlowSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var fs genkitv1alpha1.FlowSet
	if err := r.Get(ctx, req.NamespacedName, &fs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := fs.DeepCopy()
	fs.Status.ObservedGeneration = fs.Generation
	fs.Status.TotalFlows = int32(len(fs.Spec.Flows))

	if fs.Spec.Image == "" {
		setCondition(&fs.Status.Conditions, notReadyCondition(
			fs.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.image is required",
		))
		fs.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &fs)
	}

	deps, depErr := r.resolveDeps(ctx, &fs)
	if depErr != nil {
		setCondition(&fs.Status.Conditions, notReadyCondition(
			fs.Generation, genkitv1alpha1.ReasonMissingDependency, depErr.Error(),
		))
		fs.Status.Phase = genkitv1alpha1.FlowPhasePending
		fs.Status.Flows = perFlowPending(&fs, depErr.Error())
		return r.patchStatus(ctx, original, &fs)
	}

	rendered, err := renderFlowSet(&fs, deps)
	if err != nil {
		log.Error(err, "render failed")
		setCondition(&fs.Status.Conditions, notReadyCondition(
			fs.Generation, genkitv1alpha1.ReasonError, err.Error(),
		))
		fs.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &fs)
	}

	if err := r.applyChildren(ctx, &fs, rendered); err != nil {
		log.Error(err, "apply children failed")
		setCondition(&fs.Status.Conditions, notReadyCondition(
			fs.Generation, genkitv1alpha1.ReasonError, err.Error(),
		))
		fs.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &fs)
	}

	fs.Status.ContentHash = rendered.contentHash

	// Garbage-collect orphan per-flow ConfigMaps from previous reconciles.
	if err := r.gcOrphans(ctx, &fs, rendered); err != nil {
		log.Error(err, "gc orphans failed")
	}

	// Reflect the underlying Deployment status onto FlowSet status.
	var dep appsv1.Deployment
	getErr := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: fs.Name}, &dep)
	switch {
	case getErr == nil:
		fs.Status.AvailableReplicas = dep.Status.AvailableReplicas
		fs.Status.Phase = deriveFlowSetPhase(&fs, &dep)
		ready := fs.Status.Phase == genkitv1alpha1.FlowPhaseRunning
		var msg string
		if ready {
			msg = fmt.Sprintf("%d/%d replicas available", dep.Status.AvailableReplicas, dep.Status.Replicas)
			setCondition(&fs.Status.Conditions, readyCondition(
				fs.Generation, msg,
			))
		} else {
			msg = fmt.Sprintf("deployment phase=%s, %d/%d replicas available",
				fs.Status.Phase, dep.Status.AvailableReplicas, dep.Status.Replicas)
			setCondition(&fs.Status.Conditions, notReadyCondition(
				fs.Generation, genkitv1alpha1.ReasonReconciling, msg,
			))
		}
		fs.Status.Flows = perFlowFromDeployment(&fs, ready, fs.Status.Phase, msg)
	case apierrors.IsNotFound(getErr):
		fs.Status.Phase = genkitv1alpha1.FlowPhasePending
		setCondition(&fs.Status.Conditions, notReadyCondition(
			fs.Generation, genkitv1alpha1.ReasonReconciling, "waiting for deployment",
		))
		fs.Status.Flows = perFlowPending(&fs, "waiting for deployment")
	default:
		return ctrl.Result{}, getErr
	}

	// Recount readiness.
	var ready int32
	for _, e := range fs.Status.Flows {
		if e.Ready {
			ready++
		}
	}
	fs.Status.ReadyFlows = ready

	return r.patchStatus(ctx, original, &fs)
}

// resolveDeps fetches every Prompt/Tool/Model/PluginConfig/Secret for
// every flow in the set.
func (r *FlowSetReconciler) resolveDeps(ctx context.Context, fs *genkitv1alpha1.FlowSet) (*resolvedFlowSet, error) {
	out := &resolvedFlowSet{flows: make([]resolvedFlowSetFlow, 0, len(fs.Spec.Flows))}
	seen := map[string]struct{}{}
	for _, tmpl := range fs.Spec.Flows {
		if _, dup := seen[tmpl.Name]; dup {
			return nil, fmt.Errorf("duplicate flow name %q", tmpl.Name)
		}
		seen[tmpl.Name] = struct{}{}

		rf := resolvedFlowSetFlow{template: tmpl}

		for _, ref := range tmpl.Prompts {
			var p genkitv1alpha1.Prompt
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: ref.Name}, &p); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: prompt %q not found", tmpl.Name, ref.Name)
				}
				return nil, err
			}
			rf.prompts = append(rf.prompts, p)
		}
		for _, ref := range tmpl.Tools {
			var t genkitv1alpha1.Tool
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: ref.Name}, &t); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: tool %q not found", tmpl.Name, ref.Name)
				}
				return nil, err
			}
			rf.tools = append(rf.tools, t)
		}
		if tmpl.ModelRef.Name == "" {
			return nil, fmt.Errorf("flow %q: modelRef.name is required", tmpl.Name)
		}
		var m genkitv1alpha1.Model
		if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: tmpl.ModelRef.Name}, &m); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("flow %q: model %q not found", tmpl.Name, tmpl.ModelRef.Name)
			}
			return nil, err
		}
		rf.model = m

		var pc genkitv1alpha1.PluginConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: m.Spec.PluginConfigRef.Name}, &pc); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("flow %q: pluginconfig %q not found (referenced by model %q)",
					tmpl.Name, m.Spec.PluginConfigRef.Name, m.Name)
			}
			return nil, err
		}
		rf.plugin = pc

		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("flow %q: secret %q not found (referenced by pluginconfig %q)",
					tmpl.Name, pc.Spec.CredentialsRef.Name, pc.Name)
			}
			return nil, err
		}
		rf.secretName = secret.Name
		rf.secret = &secret

		out.flows = append(out.flows, rf)
	}
	return out, nil
}

// applyChildren applies every rendered object via Server-Side Apply.
func (r *FlowSetReconciler) applyChildren(ctx context.Context, fs *genkitv1alpha1.FlowSet, rfs *renderedFlowSet) error {
	objs := make([]client.Object, 0, 2+len(rfs.perFlowCMs)+1)
	objs = append(objs, rfs.manifestCM)
	for _, cm := range rfs.perFlowCMs {
		objs = append(objs, cm)
	}
	objs = append(objs, rfs.service, rfs.deploy)

	for _, obj := range objs {
		if err := controllerutil.SetControllerReference(fs, obj, r.Scheme); err != nil {
			return err
		}
		//nolint:staticcheck // SSA via Patch is still the supported path for client.Object;
		// the new client.Apply requires typed runtime.ApplyConfiguration generated code.
		if err := r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldManagerName)); err != nil {
			return fmt.Errorf("apply %T %s: %w", obj, obj.GetName(), err)
		}
	}
	return nil
}

// gcOrphans deletes ConfigMaps owned by this FlowSet that are no longer
// referenced by the rendered set (e.g. a flow was removed from spec).
func (r *FlowSetReconciler) gcOrphans(ctx context.Context, fs *genkitv1alpha1.FlowSet, rfs *renderedFlowSet) error {
	want := map[string]struct{}{
		rfs.manifestCM.Name: {},
	}
	for _, cm := range rfs.perFlowCMs {
		want[cm.Name] = struct{}{}
	}
	var cms corev1.ConfigMapList
	if err := r.List(ctx, &cms,
		client.InNamespace(fs.Namespace),
		client.MatchingLabels{genkitv1alpha1.LabelFlowSet: fs.Name},
	); err != nil {
		return err
	}
	for i := range cms.Items {
		cm := &cms.Items[i]
		if !metav1.IsControlledBy(cm, fs) {
			continue
		}
		if _, keep := want[cm.Name]; keep {
			continue
		}
		if err := r.Delete(ctx, cm); err != nil && client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func deriveFlowSetPhase(fs *genkitv1alpha1.FlowSet, dep *appsv1.Deployment) genkitv1alpha1.FlowPhase {
	desired := int32(1)
	if fs.Spec.Replicas != nil {
		desired = *fs.Spec.Replicas
	}
	if desired == 0 {
		return genkitv1alpha1.FlowPhaseRunning
	}
	if dep.Status.UnavailableReplicas > 0 && dep.Status.AvailableReplicas == 0 {
		return genkitv1alpha1.FlowPhasePending
	}
	if dep.Generation != dep.Status.ObservedGeneration {
		return genkitv1alpha1.FlowPhaseUpdating
	}
	if dep.Status.AvailableReplicas >= desired {
		return genkitv1alpha1.FlowPhaseRunning
	}
	return genkitv1alpha1.FlowPhaseUpdating
}

func perFlowFromDeployment(fs *genkitv1alpha1.FlowSet, ready bool, phase genkitv1alpha1.FlowPhase, msg string) []genkitv1alpha1.FlowStatusEntry {
	out := make([]genkitv1alpha1.FlowStatusEntry, 0, len(fs.Spec.Flows))
	for _, f := range fs.Spec.Flows {
		out = append(out, genkitv1alpha1.FlowStatusEntry{
			Name: f.Name, Ready: ready, Phase: phase, Message: msg,
		})
	}
	return out
}

func perFlowPending(fs *genkitv1alpha1.FlowSet, msg string) []genkitv1alpha1.FlowStatusEntry {
	out := make([]genkitv1alpha1.FlowStatusEntry, 0, len(fs.Spec.Flows))
	for _, f := range fs.Spec.Flows {
		out = append(out, genkitv1alpha1.FlowStatusEntry{
			Name: f.Name, Ready: false, Phase: genkitv1alpha1.FlowPhasePending, Message: msg,
		})
	}
	return out
}

func (r *FlowSetReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.FlowSet) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller. The FlowSet owns its
// Deployment, Service, and per-flow ConfigMaps; changes to referenced
// Prompts/Tools/Models trigger reconciliation via the dependency mapper.
func (r *FlowSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.FlowSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&genkitv1alpha1.Prompt{}, flowSetDependencyMapper(r.Client, "Prompt")).
		Watches(&genkitv1alpha1.Tool{}, flowSetDependencyMapper(r.Client, "Tool")).
		Watches(&genkitv1alpha1.Model{}, flowSetDependencyMapper(r.Client, "Model")).
		Watches(&genkitv1alpha1.PluginConfig{}, pluginConfigToFlowSetMapper(r.Client)).
		Watches(&corev1.Secret{}, secretToFlowSetMapper(r.Client)).
		Named("flowset").
		Complete(r)
}
