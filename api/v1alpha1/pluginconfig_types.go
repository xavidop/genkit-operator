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

// PluginConfigSpec defines the desired state of PluginConfig.
//
// A PluginConfig describes a Genkit model provider (vertexai, googleai,
// openai, anthropic, bedrock, ollama, ...). It bundles the credentials
// reference plus any provider-specific configuration that downstream Model
// CRs need.
type PluginConfigSpec struct {
	// Type is the provider type identifier. Must match a plugin known to the
	// runtime container (e.g. "vertexai", "googleai", "openai", "anthropic",
	// "bedrock", "ollama").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`

	// Region is the optional cloud region for the provider.
	// +optional
	Region string `json:"region,omitempty"`

	// CredentialsRef references a Secret in the same namespace that holds
	// the provider credentials. Its keys are exposed to the Flow container
	// via envFrom (single-Flow Pods) or as files under
	// /genkit/flows/<flow>/credentials/ (FlowSet Pods).
	// +kubebuilder:validation:Required
	CredentialsRef corev1.LocalObjectReference `json:"credentialsRef"`

	// CredentialKeys names the Secret keys this plugin expects to consume
	// (e.g. ["ANTHROPIC_API_KEY"], ["OPENAI_API_KEY"],
	// ["GOOGLE_APPLICATION_CREDENTIALS"]). When empty, the runtime falls
	// back to the per-plugin defaults documented in
	// docs/runtime-contract.md. The controller propagates this list to
	// the runtime via config.json so the runner image does not need to
	// hard-code per-provider key names.
	// +optional
	CredentialKeys []string `json:"credentialKeys,omitempty"`

	// ExtraConfig is a free-form JSON object for provider-specific knobs
	// such as baseURL, projectID, organization, etc.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ExtraConfig *apiextensionsv1.JSON `json:"extraConfig,omitempty"`
}

// PluginConfigStatus defines the observed state of PluginConfig.
type PluginConfigStatus struct {
	// Conditions reflect the current state of the PluginConfig.
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
// +kubebuilder:resource:scope=Namespaced,shortName=gpc
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PluginConfig is the Schema for the pluginconfigs API.
type PluginConfig struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec PluginConfigSpec `json:"spec"`
	// +optional
	Status PluginConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PluginConfigList contains a list of PluginConfig.
type PluginConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PluginConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PluginConfig{}, &PluginConfigList{})
}
