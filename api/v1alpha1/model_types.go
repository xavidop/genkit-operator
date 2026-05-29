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

// ModelSpec defines the desired state of Model.
//
// A Model wraps a (provider, model-id) pair and an optional declaration of
// the model's capabilities (genkit-go ai.ModelInfo) and default generation
// configuration (genkit-go ai.GenerationCommonConfig). These are stored as
// raw JSON so any Genkit runtime can consume them without conversion.
type ModelSpec struct {
	// Provider is the provider identifier; must match a PluginConfig.spec.type
	// (e.g. "anthropic", "googleai", "vertexai", "openai", "ollama").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Model is the provider's model identifier
	// (e.g. "claude-opus-4-7", "gemini-2.0-flash", "gpt-4o").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// PluginConfigRef references the PluginConfig that supplies credentials
	// and provider configuration for this model.
	// +kubebuilder:validation:Required
	PluginConfigRef corev1.LocalObjectReference `json:"pluginConfigRef"`

	// Info mirrors genkit-go's ai.ModelInfo: capability flags such as
	// multiturn, media, tools, systemRole, plus label/stage/versions.
	// Stored as raw JSON to avoid coupling the CRD schema to a specific
	// genkit-go minor version.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Info *apiextensionsv1.JSON `json:"info,omitempty"`

	// DefaultConfig mirrors genkit-go's ai.GenerationCommonConfig:
	// temperature, maxOutputTokens, topP, topK, stopSequences, version.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	DefaultConfig *apiextensionsv1.JSON `json:"defaultConfig,omitempty"`
}

// ModelStatus defines the observed state of Model.
type ModelStatus struct {
	// Conditions reflect the current state of the Model. Ready reflects
	// whether the referenced PluginConfig exists and is itself Ready.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gmd
// +kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.provider`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.model`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Model is the Schema for the models API.
type Model struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ModelSpec `json:"spec"`
	// +optional
	Status ModelStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model.
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
