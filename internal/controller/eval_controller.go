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
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// EvalReconciler reconciles Eval resources by reconciling a Job or
// CronJob that runs the eval runner image with mounts/env that point at
// the target Flow, Dataset, metric list, and output sink.
type EvalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=evals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=evals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=evals/finalizers,verbs=update
// +kubebuilder:rbac:groups=genkit.dev,resources=flows;datasets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile resolves Flow and Dataset, creates a Job or CronJob, and
// surfaces the latest result on status.
func (r *EvalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var e genkitv1alpha1.Eval
	if err := r.Get(ctx, req.NamespacedName, &e); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	original := e.DeepCopy()
	e.Status.ObservedGeneration = e.Generation

	// Resolve Flow + Dataset.
	var flow genkitv1alpha1.Flow
	if err := r.Get(ctx, types.NamespacedName{Namespace: e.Namespace, Name: e.Spec.FlowRef.Name}, &flow); err != nil {
		if apierrors.IsNotFound(err) {
			setCondition(&e.Status.Conditions, notReadyCondition(
				e.Generation, genkitv1alpha1.ReasonMissingDependency,
				fmt.Sprintf("flow %q not found", e.Spec.FlowRef.Name),
			))
			return r.patchStatus(ctx, original, &e)
		}
		return ctrl.Result{}, err
	}
	var ds genkitv1alpha1.Dataset
	if err := r.Get(ctx, types.NamespacedName{Namespace: e.Namespace, Name: e.Spec.DatasetRef.Name}, &ds); err != nil {
		if apierrors.IsNotFound(err) {
			setCondition(&e.Status.Conditions, notReadyCondition(
				e.Generation, genkitv1alpha1.ReasonMissingDependency,
				fmt.Sprintf("dataset %q not found", e.Spec.DatasetRef.Name),
			))
			return r.patchStatus(ctx, original, &e)
		}
		return ctrl.Result{}, err
	}
	if err := validateSink(e.Spec.OutputSink); err != nil {
		setCondition(&e.Status.Conditions, notReadyCondition(
			e.Generation, genkitv1alpha1.ReasonInvalidSpec, err.Error(),
		))
		return r.patchStatus(ctx, original, &e)
	}

	podSpec := evalPodSpec(&e, &flow, &ds)

	if e.Spec.Schedule != "" {
		cj := newCronJob(&e, podSpec)
		if err := controllerutil.SetControllerReference(&e, cj, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		//nolint:staticcheck // SSA via Patch is still supported for client.Object.
		if err := r.Patch(ctx, cj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldManagerName)); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		job := newJob(&e, podSpec)
		if err := controllerutil.SetControllerReference(&e, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		// Only create if not present - Jobs are immutable.
		var existing batchv1.Job
		err := r.Get(ctx, types.NamespacedName{Namespace: job.Namespace, Name: job.Name}, &existing)
		switch {
		case apierrors.IsNotFound(err):
			if err := r.Create(ctx, job); err != nil {
				return ctrl.Result{}, err
			}
			e.Status.ActiveJob = job.Name
		case err != nil:
			return ctrl.Result{}, err
		default:
			r.observeJob(&e, &existing)
		}
	}

	// Surface ConfigMap-sink results, if available.
	if e.Spec.OutputSink.ConfigMap != nil {
		var cm corev1.ConfigMap
		err := r.Get(ctx, types.NamespacedName{Namespace: e.Namespace, Name: e.Spec.OutputSink.ConfigMap.Name}, &cm)
		if err == nil {
			if raw, ok := cm.Data[e.Spec.OutputSink.ConfigMap.Key]; ok && raw != "" {
				if json.Valid([]byte(raw)) {
					e.Status.LastRunResult = &apiextensionsv1.JSON{Raw: []byte(raw)}
				}
			}
		}
	}

	setCondition(&e.Status.Conditions, readyCondition(
		e.Generation,
		"eval workload reconciled",
	))
	return r.patchStatus(ctx, original, &e)
}

func validateSink(s genkitv1alpha1.EvalOutputSink) error {
	n := 0
	if s.ConfigMap != nil {
		n++
	}
	if s.S3 != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("spec.outputSink must set exactly one of configMap or s3")
	}
	return nil
}

// evalPodSpec returns the eval runner Pod spec. The runner reads
// EVAL_FLOW_URL, EVAL_DATASET_*, and EVAL_METRICS from the environment.
func evalPodSpec(e *genkitv1alpha1.Eval, flow *genkitv1alpha1.Flow, ds *genkitv1alpha1.Dataset) corev1.PodSpec {
	port := int32(8080)
	if flow.Spec.Port != nil {
		port = *flow.Spec.Port
	}
	flowURL := fmt.Sprintf("http://%s.%s.svc:%d", flow.Name, flow.Namespace, port)
	env := []corev1.EnvVar{
		{Name: "EVAL_FLOW_URL", Value: flowURL},
		{Name: "EVAL_DATASET_NAME", Value: ds.Name},
		{Name: "EVAL_DATASET_FORMAT", Value: string(ds.Spec.Format)},
		{Name: "EVAL_METRICS", Value: strings.Join(e.Spec.Metrics, ",")},
	}
	if ds.Spec.Source.URI != "" {
		env = append(env, corev1.EnvVar{Name: "EVAL_DATASET_URI", Value: ds.Spec.Source.URI})
	}
	if ds.Spec.Source.ConfigMapRef != nil {
		env = append(env, corev1.EnvVar{Name: "EVAL_DATASET_CONFIGMAP", Value: ds.Spec.Source.ConfigMapRef.Name})
	}
	if e.Spec.OutputSink.ConfigMap != nil {
		env = append(env,
			corev1.EnvVar{Name: "EVAL_SINK_KIND", Value: "configmap"},
			corev1.EnvVar{Name: "EVAL_SINK_NAME", Value: e.Spec.OutputSink.ConfigMap.Name},
			corev1.EnvVar{Name: "EVAL_SINK_KEY", Value: e.Spec.OutputSink.ConfigMap.Key},
		)
	}
	if e.Spec.OutputSink.S3 != nil {
		env = append(env,
			corev1.EnvVar{Name: "EVAL_SINK_KIND", Value: "s3"},
			corev1.EnvVar{Name: "EVAL_SINK_BUCKET", Value: e.Spec.OutputSink.S3.Bucket},
			corev1.EnvVar{Name: "EVAL_SINK_PREFIX", Value: e.Spec.OutputSink.S3.Prefix},
		)
	}
	container := corev1.Container{
		Name:  "runner",
		Image: e.Spec.RunnerImage,
		Env:   env,
	}
	if e.Spec.OutputSink.S3 != nil {
		container.EnvFrom = []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: e.Spec.OutputSink.S3.CredentialsRef.Name,
				},
			},
		}}
	}
	return corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{container},
	}
}

func newJob(e *genkitv1alpha1.Eval, pod corev1.PodSpec) *batchv1.Job {
	parallelism := int32(1)
	if e.Spec.Concurrency != nil {
		parallelism = *e.Spec.Concurrency
	}
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.Name + "-run",
			Namespace: e.Namespace,
			Labels:    managedLabels(map[string]string{"genkit.dev/eval": e.Name}),
		},
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: managedLabels(map[string]string{"genkit.dev/eval": e.Name}),
				},
				Spec: pod,
			},
		},
	}
}

func newCronJob(e *genkitv1alpha1.Eval, pod corev1.PodSpec) *batchv1.CronJob {
	parallelism := int32(1)
	if e.Spec.Concurrency != nil {
		parallelism = *e.Spec.Concurrency
	}
	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.Name,
			Namespace: e.Namespace,
			Labels:    managedLabels(map[string]string{"genkit.dev/eval": e.Name}),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: e.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Parallelism: &parallelism,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: managedLabels(map[string]string{"genkit.dev/eval": e.Name}),
						},
						Spec: pod,
					},
				},
			},
		},
	}
}

func (r *EvalReconciler) observeJob(e *genkitv1alpha1.Eval, job *batchv1.Job) {
	if job.Status.CompletionTime != nil {
		e.Status.LastRunTime = job.Status.CompletionTime
		e.Status.ActiveJob = ""
	} else if job.Status.Active > 0 {
		e.Status.ActiveJob = job.Name
	}
}

func (r *EvalReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Eval) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with Job and CronJob ownership.
func (r *EvalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Eval{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named("eval").
		Complete(r)
}
