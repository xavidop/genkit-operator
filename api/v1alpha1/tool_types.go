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

// HTTPMethod enumerates the HTTP verbs supported by an HTTP-backed Tool.
// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE
type HTTPMethod string

// HTTPToolImpl describes how to call an external HTTP endpoint as a tool.
type HTTPToolImpl struct {
	// URL is the absolute endpoint URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// Method is the HTTP verb to use. Defaults to POST.
	// +kubebuilder:default=POST
	// +optional
	Method HTTPMethod `json:"method,omitempty"`

	// HeadersSecretRef optionally references a Secret whose keys/values
	// will be sent as additional HTTP headers (e.g. Authorization).
	// +optional
	HeadersSecretRef *corev1.LocalObjectReference `json:"headersSecretRef,omitempty"`
}

// ToolImplementation is a oneof: exactly one of its fields must be set.
type ToolImplementation struct {
	// FlowRef calls a Flow in the same namespace as the tool body.
	// +optional
	FlowRef *corev1.LocalObjectReference `json:"flowRef,omitempty"`

	// HTTP calls an external HTTP endpoint as the tool body.
	// +optional
	HTTP *HTTPToolImpl `json:"http,omitempty"`
}

// ToolSpec defines the desired state of Tool.
//
// A Tool is a function the model can call. Genkit supports tools-as-flows;
// a Tool may wrap a Flow CR or an external HTTP endpoint. The schema and
// metadata in Definition mirror genkit-go's ai.ToolDefinition (name,
// description, inputSchema, outputSchema).
type ToolSpec struct {
	// Definition mirrors genkit-go's ai.ToolDefinition. It is stored as a
	// raw JSON object with the standard genkit JSON tags so that any Genkit
	// runtime can consume it directly.
	//
	// Expected shape:
	//
	//	{
	//	  "name": "string",
	//	  "description": "string",
	//	  "inputSchema":  { ... JSON Schema ... },
	//	  "outputSchema": { ... JSON Schema ... },
	//	  "metadata":     { ... }
	//	}
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Definition apiextensionsv1.JSON `json:"definition"`

	// Implementation describes how the tool is executed. Exactly one of its
	// fields must be set.
	// +kubebuilder:validation:Required
	Implementation ToolImplementation `json:"implementation"`
}

// ToolStatus defines the observed state of Tool.
type ToolStatus struct {
	// Conditions reflect the current state of the Tool.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ResolvedTarget is a human-readable description of the resolved
	// implementation target (e.g. "Flow/foo" or "HTTP https://example/x").
	// +optional
	ResolvedTarget string `json:"resolvedTarget,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gtl
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.status.resolvedTarget`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tool is the Schema for the tools API.
type Tool struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ToolSpec `json:"spec"`
	// +optional
	Status ToolStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ToolList contains a list of Tool.
type ToolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Tool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tool{}, &ToolList{})
}
