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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// PluginConfigReconciler reconciles PluginConfig and ensures the
// referenced credentials Secret exists. The Ready condition reflects
// exactly that single check.
type PluginConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=pluginconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=pluginconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=pluginconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the referenced credentials Secret.
func (r *PluginConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var pc genkitv1alpha1.PluginConfig
	if err := r.Get(ctx, req.NamespacedName, &pc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := pc.DeepCopy()
	pc.Status.ObservedGeneration = pc.Generation

	if pc.Spec.CredentialsRef.Name == "" {
		setCondition(&pc.Status.Conditions, notReadyCondition(
			pc.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.credentialsRef.name is empty",
		))
		return r.patchStatus(ctx, original, &pc, nil)
	}

	var secret corev1.Secret
	err := r.Get(ctx, types.NamespacedName{
		Namespace: pc.Namespace,
		Name:      pc.Spec.CredentialsRef.Name,
	}, &secret)
	switch {
	case apierrors.IsNotFound(err):
		setCondition(&pc.Status.Conditions, notReadyCondition(
			pc.Generation, genkitv1alpha1.ReasonMissingDependency,
			fmt.Sprintf("secret %q not found", pc.Spec.CredentialsRef.Name),
		))
		return r.patchStatus(ctx, original, &pc, nil)
	case err != nil:
		log.Error(err, "failed to read credentials secret")
		setCondition(&pc.Status.Conditions, notReadyCondition(
			pc.Generation, genkitv1alpha1.ReasonError, err.Error(),
		))
		return r.patchStatus(ctx, original, &pc, err)
	}

	setCondition(&pc.Status.Conditions, readyCondition(
		pc.Generation,
		fmt.Sprintf("credentials secret %q present", secret.Name),
	))
	return r.patchStatus(ctx, original, &pc, nil)
}

func (r *PluginConfigReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.PluginConfig, srcErr error) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		if srcErr != nil {
			return ctrl.Result{}, srcErr
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, srcErr
}

// SetupWithManager registers the controller with the manager and wires up
// a Watch on Secrets so that creating/updating the referenced Secret
// requeues the corresponding PluginConfig.
func (r *PluginConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.PluginConfig{}).
		Watches(&corev1.Secret{}, secretToPluginConfigMapper(r.Client)).
		Named("pluginconfig").
		Complete(r)
}
