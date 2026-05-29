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

var _ = Describe("Dataset Controller", func() {
	const ns = "default"

	It("counts samples for an inline JSON dataset", func() {
		name := uniqueName("ds-inline")
		d := &genkitv1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.DatasetSpec{
				Format: genkitv1alpha1.DatasetFormatJSON,
				Source: genkitv1alpha1.DatasetSource{
					Inline: &apiextensionsv1.JSON{Raw: []byte(`[{"q":"a"},{"q":"b"},{"q":"c"}]`)},
				},
			},
		}
		Expect(k8sClient.Create(ctx, d)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, d) })

		r := &DatasetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Dataset{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.SampleCount).To(Equal(int64(3)))
	})

	It("marks Not Ready when source oneof is empty", func() {
		name := uniqueName("ds-empty")
		d := &genkitv1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.DatasetSpec{
				Format: genkitv1alpha1.DatasetFormatJSON,
				Source: genkitv1alpha1.DatasetSource{},
			},
		}
		Expect(k8sClient.Create(ctx, d)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, d) })

		r := &DatasetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Dataset{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})

	It("counts samples from a ConfigMap-backed dataset", func() {
		name := uniqueName("ds-cm")
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-cm", Namespace: ns},
			Data:       map[string]string{"data.json": `[{"x":1},{"x":2}]`},
		}
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, cm) })

		d := &genkitv1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.DatasetSpec{
				Format: genkitv1alpha1.DatasetFormatJSON,
				Source: genkitv1alpha1.DatasetSource{
					ConfigMapRef: &corev1.LocalObjectReference{Name: cm.Name},
				},
			},
		}
		Expect(k8sClient.Create(ctx, d)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, d) })

		r := &DatasetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Dataset{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.SampleCount).To(Equal(int64(2)))
	})
})
