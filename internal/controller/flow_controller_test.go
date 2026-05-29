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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// makeReadyModel creates a Ready Model (with backing PluginConfig + Secret).
func makeReadyModel(ns, name string) *genkitv1alpha1.Model {
	pc := makeReadyPluginConfig(ns, name+"-pc")
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
	return got
}

// makeReadyPrompt creates a Prompt in the "default" namespace and runs
// the PromptReconciler so the resource has populated status (mirrors what
// production controllers expect to find).
func makeReadyPrompt(name, content string) *genkitv1alpha1.Prompt {
	const ns = "default"
	p := &genkitv1alpha1.Prompt{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       genkitv1alpha1.PromptSpec{Content: content},
	}
	Expect(k8sClient.Create(ctx, p)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, p) })
	r := &PromptReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
	Expect(err).NotTo(HaveOccurred())
	got := &genkitv1alpha1.Prompt{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
	return got
}

var _ = Describe("Flow Controller", func() {
	const ns = "default"

	It("renders Deployment, Service, and ConfigMaps on happy path", func() {
		name := uniqueName("flow-ok")
		m := makeReadyModel(ns, name+"-model")
		p := makeReadyPrompt(name+"-prompt", "---\nmodel: x\n---\nhi")

		flow := &genkitv1alpha1.Flow{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSpec{
				Image:    "ghcr.io/example/flow:1",
				ModelRef: &corev1.LocalObjectReference{Name: m.Name},
				Prompts:  []corev1.LocalObjectReference{{Name: p.Name}},
			},
		}
		Expect(k8sClient.Create(ctx, flow)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, flow) })

		r := &FlowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).To(HaveKey(genkitv1alpha1.AnnotationContentHash))
		var svc corev1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &svc)).To(Succeed())
		var cm corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-prompts"}, &cm)).To(Succeed())
		Expect(cm.Data).To(HaveKey(p.Name + ".prompt"))
	})

	It("is Not Ready when a referenced Prompt is missing", func() {
		name := uniqueName("flow-noprompt")
		flow := &genkitv1alpha1.Flow{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSpec{
				Image:   "ghcr.io/example/flow:1",
				Prompts: []corev1.LocalObjectReference{{Name: "nope"}},
			},
		}
		Expect(k8sClient.Create(ctx, flow)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, flow) })

		r := &FlowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.Flow{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionFalse))
	})

	It("changes content hash when a referenced Prompt body changes", func() {
		name := uniqueName("flow-hash")
		p := makeReadyPrompt(name+"-prompt", "---\nm: a\n---\nv1")

		flow := &genkitv1alpha1.Flow{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSpec{
				Image:   "ghcr.io/example/flow:1",
				Prompts: []corev1.LocalObjectReference{{Name: p.Name}},
			},
		}
		Expect(k8sClient.Create(ctx, flow)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, flow) })

		r := &FlowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())
		got1 := &genkitv1alpha1.Flow{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got1)).To(Succeed())
		hash1 := got1.Status.ContentHash
		Expect(hash1).NotTo(BeEmpty())

		// Mutate the prompt content; the rendered ConfigMap hash should change.
		p.Spec.Content = "---\nm: a\n---\nv2"
		Expect(k8sClient.Update(ctx, p)).To(Succeed())
		pr := &PromptReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err = pr.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: p.Name}})
		Expect(err).NotTo(HaveOccurred())

		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())
		got2 := &genkitv1alpha1.Flow{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got2)).To(Succeed())
		Expect(got2.Status.ContentHash).NotTo(Equal(hash1))
	})
})
