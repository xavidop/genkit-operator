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

// FlowPhase summarizes the high-level lifecycle state of a Flow.
// +kubebuilder:validation:Enum=Pending;Running;Failed;Updating
type FlowPhase string

const (
	FlowPhasePending  FlowPhase = "Pending"
	FlowPhaseRunning  FlowPhase = "Running"
	FlowPhaseFailed   FlowPhase = "Failed"
	FlowPhaseUpdating FlowPhase = "Updating"
)

// ServiceType mirrors a subset of corev1.ServiceType for OpenAPI clarity.
// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
type ServiceType string

// PullPolicy is the image pull policy used for the Flow Pod.
// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
type PullPolicy string

// FlowSpec defines the desired state of Flow.
//
// A Flow reconciles to a Deployment + Service plus three ConfigMaps that
// implement the operator's runtime contract documented in
// docs/runtime-contract.md.
type FlowSpec struct {
	// Image is the container image for the Flow runtime. Required when
	// this FlowSpec is rendered for a standalone Flow (it may be omitted
	// from a FlowSet defaults block and supplied per-flow).
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy controls how the kubelet pulls the image.
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy PullPolicy `json:"imagePullPolicy,omitempty"`

	// Replicas is the desired replica count for the underlying Deployment.
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

	// Prompts is the list of Prompt CRs to mount under /genkit/prompts.
	// +optional
	Prompts []corev1.LocalObjectReference `json:"prompts,omitempty"`

	// Tools is the list of Tool CRs to expose under /genkit/tools.
	// +optional
	Tools []corev1.LocalObjectReference `json:"tools,omitempty"`

	// ModelRef is the default Model for this Flow.
	// +optional
	ModelRef *corev1.LocalObjectReference `json:"modelRef,omitempty"`

	// Env is appended to the container env list.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources are the Pod's resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ServiceType controls the Service that fronts the Flow.
	// +kubebuilder:default=ClusterIP
	// +optional
	ServiceType ServiceType `json:"serviceType,omitempty"`
}

// FlowStatus defines the observed state of Flow.
type FlowStatus struct {
	// Phase is a high-level lifecycle summary.
	// +optional
	Phase FlowPhase `json:"phase,omitempty"`

	// Conditions reflect the current state of the Flow.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AvailableReplicas mirrors the underlying Deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration is the generation reflected by the current status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ContentHash is the SHA256 of the rendered runtime payload that was
	// last applied to the Pod template.
	// +optional
	ContentHash string `json:"contentHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gfl
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableReplicas`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Flow is the Schema for the flows API.
type Flow struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec FlowSpec `json:"spec"`
	// +optional
	Status FlowStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FlowList contains a list of Flow.
type FlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Flow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Flow{}, &FlowList{})
}
