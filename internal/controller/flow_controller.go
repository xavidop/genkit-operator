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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// FlowReconciler reconciles Flow resources. It is the largest controller
// in v0: it resolves Prompt/Tool/Model/PluginConfig dependencies, renders
// three ConfigMaps + a Deployment + a Service, then mirrors Deployment
// status back onto the Flow.
type FlowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=flows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=flows/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=flows/finalizers,verbs=update
// +kubebuilder:rbac:groups=genkit.dev,resources=prompts;tools;models;pluginconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;services;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile resolves the Flow's dependencies and reconciles the rendered
// child objects.
func (r *FlowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var flow genkitv1alpha1.Flow
	if err := r.Get(ctx, req.NamespacedName, &flow); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := flow.DeepCopy()
	flow.Status.ObservedGeneration = flow.Generation

	if flow.Spec.Image == "" {
		setCondition(&flow.Status.Conditions, notReadyCondition(
			flow.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.image is required",
		))
		flow.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &flow)
	}

	deps, depErr := r.resolveDeps(ctx, &flow)
	if depErr != nil {
		setCondition(&flow.Status.Conditions, notReadyCondition(
			flow.Generation, genkitv1alpha1.ReasonMissingDependency, depErr.Error(),
		))
		flow.Status.Phase = genkitv1alpha1.FlowPhasePending
		// Persist status and requeue via re-resolution on dep events.
		return r.patchStatus(ctx, original, &flow)
	}

	rendered, err := renderFlow(&flow, deps)
	if err != nil {
		log.Error(err, "render failed")
		setCondition(&flow.Status.Conditions, notReadyCondition(
			flow.Generation, genkitv1alpha1.ReasonError, err.Error(),
		))
		flow.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &flow)
	}

	if err := r.applyChildren(ctx, &flow, rendered); err != nil {
		log.Error(err, "apply children failed")
		setCondition(&flow.Status.Conditions, notReadyCondition(
			flow.Generation, genkitv1alpha1.ReasonError, err.Error(),
		))
		flow.Status.Phase = genkitv1alpha1.FlowPhaseFailed
		return r.patchStatus(ctx, original, &flow)
	}

	flow.Status.ContentHash = rendered.contentHash

	// Reflect the underlying Deployment status onto Flow status.
	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: flow.Name}, &dep); err == nil {
		flow.Status.AvailableReplicas = dep.Status.AvailableReplicas
		flow.Status.Phase = derivePhase(&flow, &dep)
		if flow.Status.Phase == genkitv1alpha1.FlowPhaseRunning {
			setCondition(&flow.Status.Conditions, readyCondition(
				flow.Generation,
				fmt.Sprintf("%d/%d replicas available", dep.Status.AvailableReplicas, dep.Status.Replicas),
			))
		} else {
			setCondition(&flow.Status.Conditions, notReadyCondition(
				flow.Generation, genkitv1alpha1.ReasonReconciling,
				fmt.Sprintf("deployment phase=%s, %d/%d replicas available", flow.Status.Phase, dep.Status.AvailableReplicas, dep.Status.Replicas),
			))
		}
	} else if apierrors.IsNotFound(err) {
		flow.Status.Phase = genkitv1alpha1.FlowPhasePending
		setCondition(&flow.Status.Conditions, notReadyCondition(
			flow.Generation, genkitv1alpha1.ReasonReconciling, "waiting for deployment",
		))
	} else {
		return ctrl.Result{}, err
	}

	return r.patchStatus(ctx, original, &flow)
}

// resolveDeps fetches every referenced Prompt/Tool/Model/PluginConfig.
// Returns an error string suitable for the Ready condition message.
func (r *FlowReconciler) resolveDeps(ctx context.Context, flow *genkitv1alpha1.Flow) (*resolvedDeps, error) {
	out := &resolvedDeps{}
	for _, ref := range flow.Spec.Prompts {
		var p genkitv1alpha1.Prompt
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: ref.Name}, &p); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("prompt %q not found", ref.Name)
			}
			return nil, err
		}
		out.prompts = append(out.prompts, p)
	}
	for _, ref := range flow.Spec.Tools {
		var t genkitv1alpha1.Tool
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: ref.Name}, &t); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("tool %q not found", ref.Name)
			}
			return nil, err
		}
		out.tools = append(out.tools, t)
	}
	if flow.Spec.ModelRef != nil && flow.Spec.ModelRef.Name != "" {
		var m genkitv1alpha1.Model
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: flow.Spec.ModelRef.Name}, &m); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("model %q not found", flow.Spec.ModelRef.Name)
			}
			return nil, err
		}
		out.model = &m
		var pc genkitv1alpha1.PluginConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: m.Spec.PluginConfigRef.Name}, &pc); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("pluginconfig %q not found (referenced by model %q)", m.Spec.PluginConfigRef.Name, m.Name)
			}
			return nil, err
		}
		out.plugin = &pc
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("secret %q not found (referenced by pluginconfig %q)", pc.Spec.CredentialsRef.Name, pc.Name)
			}
			return nil, err
		}
		out.secret = &secret
	}
	return out, nil
}

func (r *FlowReconciler) applyChildren(ctx context.Context, flow *genkitv1alpha1.Flow, rf *renderedFlow) error {
	for _, obj := range []client.Object{rf.promptsCM, rf.toolsCM, rf.configCM, rf.service, rf.deploy} {
		if err := controllerutil.SetControllerReference(flow, obj, r.Scheme); err != nil {
			return err
		}
		if err := r.applyObject(ctx, obj); err != nil {
			return fmt.Errorf("apply %T %s: %w", obj, obj.GetName(), err)
		}
	}
	return nil
}

// applyObject performs a Server-Side Apply with the operator's field
// manager. Used for every generated child object.
func (r *FlowReconciler) applyObject(ctx context.Context, obj client.Object) error {
	//nolint:staticcheck // SSA via Patch is still supported for client.Object.
	return r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldManagerName))
}

func derivePhase(flow *genkitv1alpha1.Flow, dep *appsv1.Deployment) genkitv1alpha1.FlowPhase {
	desired := int32(1)
	if flow.Spec.Replicas != nil {
		desired = *flow.Spec.Replicas
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

func (r *FlowReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Flow) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *FlowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Flow{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&genkitv1alpha1.Prompt{}, flowDependencyMapper(r.Client, "Prompt")).
		Watches(&genkitv1alpha1.Tool{}, flowDependencyMapper(r.Client, "Tool")).
		Watches(&genkitv1alpha1.Model{}, flowDependencyMapper(r.Client, "Model")).
		Watches(&genkitv1alpha1.PluginConfig{}, pluginConfigToFlowMapper(r.Client)).
		Watches(&corev1.Secret{}, secretToFlowMapper(r.Client)).
		Named("flow").
		Complete(r)
}

// ensure imports are kept; some are only used in tests or future hooks.
var _ = metav1.Time{}
var _ = apimeta.FindStatusCondition
