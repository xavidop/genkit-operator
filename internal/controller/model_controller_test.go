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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// makeReadyPluginConfig creates a PluginConfig + Secret pair and reconciles
// it so the resulting object has Ready=True.
func makeReadyPluginConfig(name string) *genkitv1alpha1.PluginConfig {
	const ns = "default"
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
	return got
}

var _ = Describe("Model Controller", func() {
	const ns = "default"

	It("is Not Ready when PluginConfig is missing", func() {
		name := uniqueName("model-nopc")
		m := &genkitv1alpha1.Model{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.ModelSpec{
				Provider:        "anthropic",
				Model:           "claude-opus-4-7",
				PluginConfigRef: corev1.LocalObjectReference{Name: "missing-pc"},
			},
		}
		Expect(k8sClient.Create(ctx, m)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, m) })

		r := &ModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Model{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})

	It("is Ready when its PluginConfig is Ready", func() {
		name := uniqueName("model-ok")
		pc := makeReadyPluginConfig(name + "-pc")

		m := &genkitv1alpha1.Model{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.ModelSpec{
				Provider:        pc.Spec.Type,
				Model:           "claude-opus-4-7",
				PluginConfigRef: corev1.LocalObjectReference{Name: pc.Name},
			},
		}
		Expect(k8sClient.Create(ctx, m)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, m) })

		r := &ModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Model{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
	})
})
