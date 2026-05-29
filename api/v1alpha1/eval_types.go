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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapSink writes Eval run results back into a ConfigMap.
type ConfigMapSink struct {
	// Name is the ConfigMap name in the same namespace.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Key is the ConfigMap data key to write the JSON result into.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// S3Sink writes Eval run results to S3 (or an S3-compatible store).
type S3Sink struct {
	// Bucket is the destination bucket.
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`
	// Prefix is an optional key prefix for output objects.
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// CredentialsRef is a Secret in the same namespace containing AWS
	// credentials exposed to the eval runner via envFrom.
	// +kubebuilder:validation:Required
	CredentialsRef corev1.LocalObjectReference `json:"credentialsRef"`
}

// EvalOutputSink is a oneof: exactly one of its fields must be set.
type EvalOutputSink struct {
	// ConfigMap writes the result back into a ConfigMap in-cluster.
	// +optional
	ConfigMap *ConfigMapSink `json:"configMap,omitempty"`

	// S3 writes the result as a JSON object to S3.
	// +optional
	S3 *S3Sink `json:"s3,omitempty"`
}

// EvalSpec defines the desired state of Eval.
//
// An Eval runs a Flow against a Dataset using one or more named metrics.
// If Schedule is set the operator reconciles a CronJob; otherwise it
// reconciles a one-shot Job. Result parsing is delegated to the runner
// image and surfaced in Status.LastRunResult.
type EvalSpec struct {
	// FlowRef references the Flow under test.
	// +kubebuilder:validation:Required
	FlowRef corev1.LocalObjectReference `json:"flowRef"`

	// DatasetRef references the Dataset to evaluate against.
	// +kubebuilder:validation:Required
	DatasetRef corev1.LocalObjectReference `json:"datasetRef"`

	// Metrics are evaluator names passed through to genkit-go's evaluator
	// registry (e.g. "faithfulness", "answer_relevancy", "correctness").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Metrics []string `json:"metrics"`

	// Schedule is an optional cron expression. When set, the Eval is
	// reconciled to a CronJob; otherwise a one-shot Job.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Concurrency is the parallelism for the eval Job.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Concurrency *int32 `json:"concurrency,omitempty"`

	// RunnerImage is the container image of a Genkit eval runner that
	// reads the mount contract and writes results to the output sink.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RunnerImage string `json:"runnerImage"`

	// OutputSink describes where results are written. Exactly one of
	// configMap or s3 must be set.
	// +kubebuilder:validation:Required
	OutputSink EvalOutputSink `json:"outputSink"`
}

// EvalStatus defines the observed state of Eval.
type EvalStatus struct {
	// Conditions reflect the current state of the Eval.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastRunTime is when the most recent Job completed.
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// LastRunResult is the parsed metric summary of the most recent run.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LastRunResult *apiextensionsv1.JSON `json:"lastRunResult,omitempty"`

	// ActiveJob is the name of the currently-running Job, if any.
	// +optional
	ActiveJob string `json:"activeJob,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gev
// +kubebuilder:printcolumn:name="Flow",type=string,JSONPath=`.spec.flowRef.name`
// +kubebuilder:printcolumn:name="Dataset",type=string,JSONPath=`.spec.datasetRef.name`
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Active",type=string,JSONPath=`.status.activeJob`
// +kubebuilder:printcolumn:name="LastRun",type=date,JSONPath=`.status.lastRunTime`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`

// Eval is the Schema for the evals API.
type Eval struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec EvalSpec `json:"spec"`
	// +optional
	Status EvalStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EvalList contains a list of Eval.
type EvalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Eval `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Eval{}, &EvalList{})
}
