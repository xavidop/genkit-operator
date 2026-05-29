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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// ToolReconciler reconciles Tool resources. It validates that exactly one
// implementation is set and that the referenced target resolves.
type ToolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=tools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=tools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=tools/finalizers,verbs=update
// +kubebuilder:rbac:groups=genkit.dev,resources=flows,verbs=get;list;watch

// Reconcile validates Tool implementation and reflects results in status.
func (r *ToolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var t genkitv1alpha1.Tool
	if err := r.Get(ctx, req.NamespacedName, &t); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := t.DeepCopy()
	t.Status.ObservedGeneration = t.Generation

	flowSet := t.Spec.Implementation.FlowRef != nil && t.Spec.Implementation.FlowRef.Name != ""
	httpSet := t.Spec.Implementation.HTTP != nil && t.Spec.Implementation.HTTP.URL != ""

	switch {
	case flowSet && httpSet:
		setCondition(&t.Status.Conditions, notReadyCondition(
			t.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"exactly one of spec.implementation.flowRef or spec.implementation.http must be set, not both",
		))
		return r.patchStatus(ctx, original, &t)
	case !flowSet && !httpSet:
		setCondition(&t.Status.Conditions, notReadyCondition(
			t.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.implementation must set exactly one of flowRef or http",
		))
		return r.patchStatus(ctx, original, &t)
	}

	if flowSet {
		var flow genkitv1alpha1.Flow
		err := r.Get(ctx, types.NamespacedName{
			Namespace: t.Namespace,
			Name:      t.Spec.Implementation.FlowRef.Name,
		}, &flow)
		if apierrors.IsNotFound(err) {
			setCondition(&t.Status.Conditions, notReadyCondition(
				t.Generation, genkitv1alpha1.ReasonMissingDependency,
				fmt.Sprintf("flow %q not found", t.Spec.Implementation.FlowRef.Name),
			))
			return r.patchStatus(ctx, original, &t)
		}
		if err != nil {
			return ctrl.Result{}, err
		}
		t.Status.ResolvedTarget = fmt.Sprintf("Flow/%s", flow.Name)
	} else {
		t.Status.ResolvedTarget = fmt.Sprintf("HTTP %s", t.Spec.Implementation.HTTP.URL)
	}

	setCondition(&t.Status.Conditions, readyCondition(
		t.Generation,
		fmt.Sprintf("tool implementation resolved to %s", t.Status.ResolvedTarget),
	))
	return r.patchStatus(ctx, original, &t)
}

func (r *ToolReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Tool) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller and watches referenced Flows
// so Tool readiness updates when its flow target appears/disappears.
func (r *ToolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Tool{}).
		Watches(&genkitv1alpha1.Flow{}, flowToToolMapper(r.Client)).
		Named("tool").
		Complete(r)
}
