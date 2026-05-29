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

package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// fieldManagerName is the Server-Side Apply field manager used for all
// generated resources owned by this operator.
const fieldManagerName = "genkit-operator"

// readyCondition returns a Ready=True condition. The reason is always
// genkitv1alpha1.ReasonReady; only the message and observed generation
// vary per caller.
func readyCondition(generation int64, message string) metav1.Condition {
	return metav1.Condition{
		Type:               genkitv1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             genkitv1alpha1.ReasonReady,
		Message:            message,
		ObservedGeneration: generation,
	}
}

// notReadyCondition returns a Ready=False condition.
func notReadyCondition(generation int64, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               genkitv1alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
}

// setCondition is a thin wrapper around apimeta.SetStatusCondition.
func setCondition(conditions *[]metav1.Condition, c metav1.Condition) {
	apimeta.SetStatusCondition(conditions, c)
}

// sha256Hex returns the lowercase hex SHA-256 of in.
func sha256Hex(in []byte) string {
	sum := sha256.Sum256(in)
	return hex.EncodeToString(sum[:])
}

// managedLabels returns the labels every operator-managed object carries.
func managedLabels(extra map[string]string) map[string]string {
	out := map[string]string{
		genkitv1alpha1.LabelManagedBy: genkitv1alpha1.ManagedByValue,
	}
	maps.Copy(out, extra)
	return out
}
