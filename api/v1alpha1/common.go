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

// Common condition type strings used across the operator's CRDs.
const (
	// ConditionReady indicates that the resource has been successfully
	// reconciled and is ready to be used.
	ConditionReady = "Ready"

	// ReasonReconciling indicates the resource is being reconciled.
	ReasonReconciling = "Reconciling"
	// ReasonReady indicates the resource is ready.
	ReasonReady = "Ready"
	// ReasonMissingDependency indicates a required reference cannot be found.
	ReasonMissingDependency = "MissingDependency"
	// ReasonInvalidSpec indicates the spec failed validation.
	ReasonInvalidSpec = "InvalidSpec"
	// ReasonError indicates a generic reconciliation error.
	ReasonError = "Error"
)

// Annotation and label keys used by the operator.
const (
	// LabelManagedBy identifies resources managed by this operator.
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// ManagedByValue is the value used for LabelManagedBy.
	ManagedByValue = "genkit-operator"

	// LabelFlow identifies the Flow that owns a resource.
	LabelFlow = "genkit.dev/flow"
	// LabelFlowSet identifies the FlowSet that owns a Flow.
	LabelFlowSet = "genkit.dev/flowset"

	// AnnotationContentHash records the rendered content hash of the
	// resources mounted into a Flow Pod, so any change triggers a rollout.
	AnnotationContentHash = "genkit.dev/content-hash"
)
