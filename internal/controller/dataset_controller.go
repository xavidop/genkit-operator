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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// DatasetReconciler reconciles Dataset resources. It validates the source
// oneof and, where feasible, counts samples and reflects readiness.
type DatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=datasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=datasets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=datasets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile validates the Dataset source and updates status.
func (r *DatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var d genkitv1alpha1.Dataset
	if err := r.Get(ctx, req.NamespacedName, &d); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := d.DeepCopy()
	d.Status.ObservedGeneration = d.Generation

	src := d.Spec.Source
	setCount := 0
	if src.Inline != nil {
		setCount++
	}
	if src.ConfigMapRef != nil {
		setCount++
	}
	if src.URI != "" {
		setCount++
	}
	if setCount != 1 {
		setCondition(&d.Status.Conditions, notReadyCondition(
			d.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.source must set exactly one of inline, configMapRef, or uri",
		))
		return r.patchStatus(ctx, original, &d)
	}

	switch {
	case src.Inline != nil:
		n, err := countSamplesBytes(d.Spec.Format, src.Inline.Raw)
		if err != nil {
			setCondition(&d.Status.Conditions, notReadyCondition(
				d.Generation, genkitv1alpha1.ReasonInvalidSpec,
				fmt.Sprintf("inline payload could not be parsed as %s: %v", d.Spec.Format, err),
			))
			return r.patchStatus(ctx, original, &d)
		}
		d.Status.SampleCount = int64(n)

	case src.ConfigMapRef != nil:
		var cm corev1.ConfigMap
		err := r.Get(ctx, types.NamespacedName{
			Namespace: d.Namespace,
			Name:      src.ConfigMapRef.Name,
		}, &cm)
		if apierrors.IsNotFound(err) {
			setCondition(&d.Status.Conditions, notReadyCondition(
				d.Generation, genkitv1alpha1.ReasonMissingDependency,
				fmt.Sprintf("configmap %q not found", src.ConfigMapRef.Name),
			))
			return r.patchStatus(ctx, original, &d)
		}
		if err != nil {
			return ctrl.Result{}, err
		}
		payload, ok := cm.Data["data.json"]
		if !ok {
			setCondition(&d.Status.Conditions, notReadyCondition(
				d.Generation, genkitv1alpha1.ReasonInvalidSpec,
				fmt.Sprintf("configmap %q has no data.json key", cm.Name),
			))
			return r.patchStatus(ctx, original, &d)
		}
		n, err := countSamplesBytes(d.Spec.Format, []byte(payload))
		if err != nil {
			setCondition(&d.Status.Conditions, notReadyCondition(
				d.Generation, genkitv1alpha1.ReasonInvalidSpec,
				fmt.Sprintf("data.json could not be parsed as %s: %v", d.Spec.Format, err),
			))
			return r.patchStatus(ctx, original, &d)
		}
		d.Status.SampleCount = int64(n)

	case src.URI != "":
		// Opaque external source; the operator does not fetch the bytes.
		d.Status.SampleCount = -1
	}

	setCondition(&d.Status.Conditions, readyCondition(
		d.Generation,
		fmt.Sprintf("dataset source resolved (samples=%d)", d.Status.SampleCount),
	))
	return r.patchStatus(ctx, original, &d)
}

// countSamplesBytes returns the number of samples in the given payload,
// according to the declared format. JSON is expected to be a top-level
// array; JSONL counts non-blank lines; CSV counts non-header rows.
func countSamplesBytes(format genkitv1alpha1.DatasetFormat, raw []byte) (int, error) {
	switch format {
	case genkitv1alpha1.DatasetFormatJSON:
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			// Allow a single inline object to count as 1.
			var obj map[string]json.RawMessage
			if err2 := json.Unmarshal(raw, &obj); err2 == nil {
				return 1, nil
			}
			return 0, err
		}
		return len(arr), nil

	case genkitv1alpha1.DatasetFormatJSONL:
		n := 0
		for line := range strings.SplitSeq(string(raw), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			n++
		}
		return n, nil

	case genkitv1alpha1.DatasetFormatCSV:
		rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
		if err != nil {
			return 0, err
		}
		if len(rows) == 0 {
			return 0, nil
		}
		return len(rows) - 1, nil
	}
	return 0, fmt.Errorf("unsupported format %q", format)
}

func (r *DatasetReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Dataset) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller.
func (r *DatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Dataset{}).
		Named("dataset").
		Complete(r)
}
