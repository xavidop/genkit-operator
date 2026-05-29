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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

func toolDef(name string) apiextensionsv1.JSON {
	return apiextensionsv1.JSON{Raw: []byte(`{"name":"` + name + `","description":"d","inputSchema":{},"outputSchema":{}}`)}
}

var _ = Describe("Tool Controller", func() {
	const ns = "default"

	It("rejects when both flowRef and http are set", func() {
		name := uniqueName("tool-both")
		t := &genkitv1alpha1.Tool{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.ToolSpec{
				Definition: toolDef(name),
				Implementation: genkitv1alpha1.ToolImplementation{
					FlowRef: &corev1.LocalObjectReference{Name: "x"},
					HTTP:    &genkitv1alpha1.HTTPToolImpl{URL: "https://example/x"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, t) })

		r := &ToolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Tool{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})

	It("becomes Ready for an HTTP implementation", func() {
		name := uniqueName("tool-http")
		t := &genkitv1alpha1.Tool{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.ToolSpec{
				Definition: toolDef(name),
				Implementation: genkitv1alpha1.ToolImplementation{
					HTTP: &genkitv1alpha1.HTTPToolImpl{URL: "https://example/x", Method: "POST"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, t) })

		r := &ToolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Tool{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.ResolvedTarget).To(ContainSubstring("HTTP"))
	})

	It("marks missing Flow target as Not Ready", func() {
		name := uniqueName("tool-flow-missing")
		t := &genkitv1alpha1.Tool{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.ToolSpec{
				Definition: toolDef(name),
				Implementation: genkitv1alpha1.ToolImplementation{
					FlowRef: &corev1.LocalObjectReference{Name: "does-not-exist"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, t) })

		r := &ToolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Tool{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})
})
