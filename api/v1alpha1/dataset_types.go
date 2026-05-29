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

// DatasetFormat enumerates the wire formats supported for Dataset samples.
// +kubebuilder:validation:Enum=json;jsonl;csv
type DatasetFormat string

const (
	DatasetFormatJSON  DatasetFormat = "json"
	DatasetFormatJSONL DatasetFormat = "jsonl"
	DatasetFormatCSV   DatasetFormat = "csv"
)

// DatasetSource is a oneof: exactly one of its fields must be set.
type DatasetSource struct {
	// Inline holds the dataset samples directly in the CR. Use for small
	// datasets only.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Inline *apiextensionsv1.JSON `json:"inline,omitempty"`

	// ConfigMapRef references a ConfigMap in the same namespace that holds
	// the dataset payload under the "data.json" key.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// URI is an external location for the dataset: s3://..., gs://...,
	// https://...
	// +optional
	URI string `json:"uri,omitempty"`
}

// DatasetSpec defines the desired state of Dataset.
//
// A Dataset is a reference to evaluation data used by Eval CRs. The shape
// of a single sample mirrors genkit-go's evaluator sample structure.
type DatasetSpec struct {
	// Format declares the wire format of the underlying samples.
	// +kubebuilder:validation:Required
	Format DatasetFormat `json:"format"`

	// Source describes where to read the dataset from. Exactly one field
	// must be set.
	// +kubebuilder:validation:Required
	Source DatasetSource `json:"source"`

	// Schema is an optional JSON Schema describing a single sample, used
	// by the runtime for client-side validation.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Schema *apiextensionsv1.JSON `json:"schema,omitempty"`
}

// DatasetStatus defines the observed state of Dataset.
type DatasetStatus struct {
	// Conditions reflect the current state of the Dataset.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// SampleCount is the number of samples discovered. Populated for
	// inline and ConfigMap sources; -1 if the source is an opaque URI.
	// +optional
	SampleCount int64 `json:"sampleCount,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gds
// +kubebuilder:printcolumn:name="Format",type=string,JSONPath=`.spec.format`
// +kubebuilder:printcolumn:name="Samples",type=integer,JSONPath=`.status.sampleCount`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Dataset is the Schema for the datasets API.
type Dataset struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec DatasetSpec `json:"spec"`
	// +optional
	Status DatasetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DatasetList contains a list of Dataset.
type DatasetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Dataset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dataset{}, &DatasetList{})
}
