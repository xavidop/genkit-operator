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
)

// InlineModelSpec defines a model configuration inline within a Flow or
// FlowSet, as an alternative to referencing a Model CR via modelRef.
// Credentials are still supplied via a PluginConfig reference.
type InlineModelSpec struct {
	// Provider is the plugin type (e.g. "anthropic", "googleai").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Model is the provider model identifier (e.g. "claude-opus-4-7").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// PluginConfigRef references the PluginConfig that supplies credentials.
	// +kubebuilder:validation:Required
	PluginConfigRef corev1.LocalObjectReference `json:"pluginConfigRef"`

	// Info mirrors genkit-go ai.ModelInfo. Schemaless — passed as-is to the runtime.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Info *apiextensionsv1.JSON `json:"info,omitempty"`

	// DefaultConfig mirrors genkit-go ai.GenerationCommonConfig. Schemaless.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	DefaultConfig *apiextensionsv1.JSON `json:"defaultConfig,omitempty"`
}

// InlinePrompt is a named dotprompt document declared directly in a Flow or
// FlowSet without creating a Prompt CR. Name becomes the mounted filename
// (<name>.prompt); Content is the full dotprompt source.
type InlinePrompt struct {
	// Name is used as the prompt filename: <name>.prompt.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Content is the full dotprompt source (YAML frontmatter + Handlebars body).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`
}

// PromptSource is one entry in a Flow or FlowSet prompts list.
// Exactly one of promptRef or prompt must be set.
type PromptSource struct {
	// PromptRef references an existing Prompt CR by name.
	// +optional
	PromptRef *corev1.LocalObjectReference `json:"promptRef,omitempty"`

	// Prompt declares the prompt content inline (no Prompt CR required).
	// +optional
	Prompt *InlinePrompt `json:"prompt,omitempty"`
}

// GetName returns the prompt name from either the ref or the inline definition.
func (ps PromptSource) GetName() string {
	if ps.PromptRef != nil {
		return ps.PromptRef.Name
	}
	if ps.Prompt != nil {
		return ps.Prompt.Name
	}
	return ""
}
