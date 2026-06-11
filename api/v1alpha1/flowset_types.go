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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FlowSetFlow is one logical flow inside a FlowSet. All FlowSetFlows in a
// single FlowSet are served by the SAME Pod (one Deployment, one Service),
// each exposed under POST /<name>.
//
// Per-flow you choose the prompts, tools, and model; pod-level fields
// (image, replicas, port, resources, env, ...) live on FlowSetSpec.
type FlowSetFlow struct {
	// Name is the flow identifier. It becomes the HTTP route (POST /<name>)
	// and is used as a directory name under /genkit/flows/<name>/.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// Prompts is the list of prompts available to this flow. The FIRST item
	// is the entrypoint invoked by POST /<name>; the rest are helpers loaded
	// into the same per-flow genkit registry. Each entry is either a
	// promptRef (reference to a Prompt CR) or an inline prompt declaration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Prompts []PromptSource `json:"prompts"`

	// Tools is the list of Tool CRs exposed to this flow.
	// +optional
	Tools []corev1.LocalObjectReference `json:"tools,omitempty"`

	// ModelRef references an existing Model CR as the default model for this
	// flow. Mutually exclusive with modelSpec; exactly one must be set.
	// +optional
	ModelRef *corev1.LocalObjectReference `json:"modelRef,omitempty"`

	// ModelSpec declares the model configuration inline without creating a
	// Model CR. Mutually exclusive with modelRef; exactly one must be set.
	// +optional
	ModelSpec *InlineModelSpec `json:"modelSpec,omitempty"`
}

// FlowSetSpec defines the desired state of FlowSet.
//
// A FlowSet renders to a SINGLE Deployment + Service that serves every
// declared flow on the same Pod. Per-flow content (prompts, tools, model)
// is mounted under /genkit/flows/<name>/ as described in
// docs/runtime-contract.md.
type FlowSetSpec struct {
	// Image is the container image of the multi-flow runtime.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// ImagePullPolicy controls how the kubelet pulls the image.
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy PullPolicy `json:"imagePullPolicy,omitempty"`

	// Replicas is the desired replica count for the Deployment.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Port is the container port the runtime listens on.
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Env is appended to the container env. Shared by every flow in
	// this set; there is no per-flow env override.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources are the Pod's resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ServiceType controls the Service that fronts the FlowSet.
	// +kubebuilder:default=ClusterIP
	// +optional
	ServiceType ServiceType `json:"serviceType,omitempty"`

	// Flows is the list of flows served by the single shared Pod.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Flows []FlowSetFlow `json:"flows"`
}

// FlowStatusEntry summarizes one flow inside a FlowSet's status.
type FlowStatusEntry struct {
	// Name matches FlowSetSpec.Flows[].Name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Ready is true when the shared Pod is Ready AND all dependencies
	// (prompts, tools, model, plugin, secret) for this flow are resolved.
	// +optional
	Ready bool `json:"ready"`

	// Phase mirrors the high-level lifecycle phase of the shared
	// Deployment, scoped per flow for kubectl-friendly status.
	// +optional
	Phase FlowPhase `json:"phase,omitempty"`

	// Message is a short human-readable message for kubectl output.
	// +optional
	Message string `json:"message,omitempty"`
}

// FlowSetStatus defines the observed state of FlowSet.
type FlowSetStatus struct {
	// Phase is a high-level lifecycle summary of the shared Deployment.
	// +optional
	Phase FlowPhase `json:"phase,omitempty"`

	// Conditions reflect the aggregated state of the FlowSet. Ready is
	// True iff every declared flow has its dependencies resolved and the
	// shared Deployment is available.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Flows is a per-flow summary, in spec order.
	// +listType=map
	// +listMapKey=name
	// +optional
	Flows []FlowStatusEntry `json:"flows,omitempty"`

	// ReadyFlows is the number of flows whose Ready entry is True.
	// +optional
	ReadyFlows int32 `json:"readyFlows"`

	// TotalFlows is the total number of flows declared.
	// +optional
	TotalFlows int32 `json:"totalFlows"`

	// AvailableReplicas mirrors the underlying Deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ContentHash is the SHA256 of the rendered runtime payload that was
	// last applied to the Pod template.
	// +optional
	ContentHash string `json:"contentHash,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gfs
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="ReadyFlows",type=integer,JSONPath=`.status.readyFlows`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.totalFlows`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FlowSet is the Schema for the flowsets API.
type FlowSet struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec FlowSetSpec `json:"spec"`
	// +optional
	Status FlowSetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FlowSetList contains a list of FlowSet.
type FlowSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []FlowSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlowSet{}, &FlowSetList{})
}
