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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// ModelReconciler reconciles Model resources. A Model is Ready iff its
// referenced PluginConfig exists and is itself Ready.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=models/finalizers,verbs=update
// +kubebuilder:rbac:groups=genkit.dev,resources=pluginconfigs,verbs=get;list;watch

// Reconcile resolves the referenced PluginConfig and propagates readiness.
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var m genkitv1alpha1.Model
	if err := r.Get(ctx, req.NamespacedName, &m); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := m.DeepCopy()
	m.Status.ObservedGeneration = m.Generation

	if m.Spec.PluginConfigRef.Name == "" {
		setCondition(&m.Status.Conditions, notReadyCondition(
			m.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.pluginConfigRef.name is empty",
		))
		return r.patchStatus(ctx, original, &m)
	}

	var pc genkitv1alpha1.PluginConfig
	err := r.Get(ctx, types.NamespacedName{
		Namespace: m.Namespace,
		Name:      m.Spec.PluginConfigRef.Name,
	}, &pc)
	if apierrors.IsNotFound(err) {
		setCondition(&m.Status.Conditions, notReadyCondition(
			m.Generation, genkitv1alpha1.ReasonMissingDependency,
			fmt.Sprintf("pluginconfig %q not found", m.Spec.PluginConfigRef.Name),
		))
		return r.patchStatus(ctx, original, &m)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	pcReady := apimeta.FindStatusCondition(pc.Status.Conditions, genkitv1alpha1.ConditionReady)
	if pcReady == nil || pcReady.Status != metav1.ConditionTrue {
		setCondition(&m.Status.Conditions, notReadyCondition(
			m.Generation, genkitv1alpha1.ReasonMissingDependency,
			fmt.Sprintf("pluginconfig %q is not Ready", pc.Name),
		))
		return r.patchStatus(ctx, original, &m)
	}

	setCondition(&m.Status.Conditions, readyCondition(
		m.Generation,
		fmt.Sprintf("provider=%s model=%s", m.Spec.Provider, m.Spec.Model),
	))
	return r.patchStatus(ctx, original, &m)
}

func (r *ModelReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Model) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller and watches PluginConfig so a
// PluginConfig becoming Ready propagates to dependent Models.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Model{}).
		Watches(&genkitv1alpha1.PluginConfig{}, pluginConfigToModelMapper(r.Client)).
		Named("model").
		Complete(r)
}
