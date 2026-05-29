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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromptSpec defines the desired state of Prompt.
//
// A Prompt is a Genkit dotprompt: YAML frontmatter followed by a templated
// body. The full content is stored verbatim in Content and is mounted into
// Flow Pods at /genkit/prompts/<name>.prompt.
type PromptSpec struct {
	// Content is the full dotprompt source (frontmatter + body).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Description is an optional human-readable summary of the prompt.
	// +optional
	Description string `json:"description,omitempty"`
}

// PromptStatus defines the observed state of Prompt.
type PromptStatus struct {
	// Conditions reflect the current state of the Prompt.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ContentHash is the SHA256 of spec.content. Flows that mount this
	// prompt include it in their rollout-triggering content hash.
	// +optional
	ContentHash string `json:"contentHash,omitempty"`

	// ParsedFrontmatter is the parsed YAML frontmatter of the dotprompt,
	// exposed for kubectl inspection. Best-effort; missing or invalid
	// frontmatter is not a fatal error.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ParsedFrontmatter *apiextensionsv1.JSON `json:"parsedFrontmatter,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gpr
// +kubebuilder:printcolumn:name="Hash",type=string,JSONPath=`.status.contentHash`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Prompt is the Schema for the prompts API.
type Prompt struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec PromptSpec `json:"spec"`
	// +optional
	Status PromptStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PromptList contains a list of Prompt.
type PromptList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Prompt `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Prompt{}, &PromptList{})
}
