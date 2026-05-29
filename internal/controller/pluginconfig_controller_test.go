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
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// uniqueSuffix produces a process-unique short suffix for test resource
// names. Tests run inside the shared `default` namespace; using unique
// names per test avoids cross-test interference.
var nameCounter atomic.Int64

func uniqueName(prefix string) string {
	n := nameCounter.Add(1)
	return fmt.Sprintf("%s-%d", prefix, n)
}

func readyStatus(conds []metav1.Condition) metav1.ConditionStatus {
	if c := apimeta.FindStatusCondition(conds, genkitv1alpha1.ConditionReady); c != nil {
		return c.Status
	}
	return ""
}

var _ = Describe("PluginConfig Controller", func() {
	const ns = "default"

	It("marks Ready=False when credentials Secret is missing", func() {
		name := uniqueName("pc-missing")
		pc := &genkitv1alpha1.PluginConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.PluginConfigSpec{
				Type:           "anthropic",
				CredentialsRef: corev1.LocalObjectReference{Name: name + "-secret"},
			},
		}
		Expect(k8sClient.Create(ctx, pc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pc) })

		r := &PluginConfigReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.PluginConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})

	It("marks Ready=True when credentials Secret exists", func() {
		name := uniqueName("pc-ok")
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-secret", Namespace: ns},
			StringData: map[string]string{"ANTHROPIC_API_KEY": "x"},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		pc := &genkitv1alpha1.PluginConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.PluginConfigSpec{
				Type:           "anthropic",
				CredentialsRef: corev1.LocalObjectReference{Name: secret.Name},
			},
		}
		Expect(k8sClient.Create(ctx, pc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pc) })

		r := &PluginConfigReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.PluginConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
	})
})
