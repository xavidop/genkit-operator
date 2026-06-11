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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

var _ = Describe("FlowSet Controller", func() {
	const ns = "default"

	It("renders one Deployment + Service + per-flow ConfigMaps + manifest", func() {
		name := uniqueName("fs-ok")
		m := makeReadyModel(name + "-model")
		pa := makeReadyPrompt(name+"-pa", "---\nmodel: x\n---\nhola")
		pb := makeReadyPrompt(name+"-pb", "---\nmodel: x\n---\nhello")

		fs := &genkitv1alpha1.FlowSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSetSpec{
				Image:    "ghcr.io/example/runner:1",
				Replicas: ptr.To[int32](1),
				Flows: []genkitv1alpha1.FlowSetFlow{
					{
						Name:     "alpha",
						ModelRef: &corev1.LocalObjectReference{Name: m.Name},
						Prompts:  []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pa.Name}}},
					},
					{
						Name:     "beta",
						ModelRef: &corev1.LocalObjectReference{Name: m.Name},
						Prompts:  []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pb.Name}}},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, fs)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, fs) })

		r := &FlowSetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		// Single Deployment + Service for the whole FlowSet.
		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).To(HaveKey(genkitv1alpha1.AnnotationContentHash))
		Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))

		var svc corev1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &svc)).To(Succeed())

		// Manifest CM enumerates both flows.
		var manifestCM corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-manifest"}, &manifestCM)).To(Succeed())
		Expect(manifestCM.Data).To(HaveKey("manifest.json"))
		Expect(manifestCM.Data["manifest.json"]).To(ContainSubstring(`"name": "alpha"`))
		Expect(manifestCM.Data["manifest.json"]).To(ContainSubstring(`"name": "beta"`))
		Expect(manifestCM.Data["manifest.json"]).To(ContainSubstring(`"entrypoint": "` + pa.Name + `"`))

		// Per-flow ConfigMaps.
		var cmAlpha corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-alpha-prompts"}, &cmAlpha)).To(Succeed())
		Expect(cmAlpha.Data).To(HaveKey(pa.Name + ".prompt"))

		var cmAlphaCfg corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-alpha-config"}, &cmAlphaCfg)).To(Succeed())
		Expect(cmAlphaCfg.Data["config.json"]).To(ContainSubstring(`"credentialsDir"`))

		var cmBeta corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-beta-prompts"}, &cmBeta)).To(Succeed())
		Expect(cmBeta.Data).To(HaveKey(pb.Name + ".prompt"))
	})

	It("is Not Ready when a flow's Model is missing", func() {
		name := uniqueName("fs-nomodel")
		p := makeReadyPrompt(name+"-prompt", "---\nmodel: x\n---\nhi")

		fs := &genkitv1alpha1.FlowSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSetSpec{
				Image: "ghcr.io/example/runner:1",
				Flows: []genkitv1alpha1.FlowSetFlow{
					{
						Name:     "lone",
						ModelRef: &corev1.LocalObjectReference{Name: "nope"},
						Prompts:  []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: p.Name}}},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, fs)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, fs) })

		r := &FlowSetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		got := &genkitv1alpha1.FlowSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, got)).To(Succeed())
		Expect(got.Status.ReadyFlows).To(BeNumerically("==", 0))
		Expect(conditionStatus(got.Status.Conditions, genkitv1alpha1.ConditionReady)).To(Equal(metav1.ConditionFalse))
	})

	It("garbage-collects per-flow ConfigMaps when a flow is removed", func() {
		name := uniqueName("fs-gc")
		m := makeReadyModel(name + "-model")
		pa := makeReadyPrompt(name+"-pa", "---\nmodel: x\n---\na")
		pb := makeReadyPrompt(name+"-pb", "---\nmodel: x\n---\nb")

		fs := &genkitv1alpha1.FlowSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSetSpec{
				Image: "ghcr.io/example/runner:1",
				Flows: []genkitv1alpha1.FlowSetFlow{
					{Name: "alpha", ModelRef: &corev1.LocalObjectReference{Name: m.Name},
						Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pa.Name}}}},
					{Name: "beta", ModelRef: &corev1.LocalObjectReference{Name: m.Name},
						Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pb.Name}}}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, fs)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, fs) })

		r := &FlowSetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		// Confirm beta CMs exist.
		var cm corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-beta-prompts"}, &cm)).To(Succeed())

		// Remove beta.
		latest := &genkitv1alpha1.FlowSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, latest)).To(Succeed())
		latest.Spec.Flows = latest.Spec.Flows[:1]
		Expect(k8sClient.Update(ctx, latest)).To(Succeed())

		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-beta-prompts"}, &cm)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("renders per-flow ConfigMaps using inline modelSpec and inline prompt", func() {
		name := uniqueName("fs-inline")
		// one flow uses a ref, the other uses inline model + inline prompt
		m := makeReadyModel(name + "-model")
		pa := makeReadyPrompt(name+"-pa", "---\nmodel: x\n---\nhola")
		pc := makeReadyPluginConfig(name + "-inline-pc")

		inlineContent := "---\nmodel: x\n---\nhello inline"

		fs := &genkitv1alpha1.FlowSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.FlowSetSpec{
				Image:    "ghcr.io/example/runner:1",
				Replicas: ptr.To[int32](1),
				Flows: []genkitv1alpha1.FlowSetFlow{
					{
						Name:     "ref-flow",
						ModelRef: &corev1.LocalObjectReference{Name: m.Name},
						Prompts:  []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pa.Name}}},
					},
					{
						Name: "inline-flow",
						ModelSpec: &genkitv1alpha1.InlineModelSpec{
							Provider:        pc.Spec.Type,
							Model:           "claude-opus-4-7",
							PluginConfigRef: corev1.LocalObjectReference{Name: pc.Name},
						},
						Prompts: []genkitv1alpha1.PromptSource{
							{Prompt: &genkitv1alpha1.InlinePrompt{
								Name:    "greeting",
								Content: inlineContent,
							}},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, fs)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, fs) })

		r := &FlowSetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		// Deployment and Service must exist.
		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).To(HaveKey(genkitv1alpha1.AnnotationContentHash))
		var svc corev1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &svc)).To(Succeed())

		// ref-flow: prompt from CR.
		var cmRefPrompts corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-ref-flow-prompts"}, &cmRefPrompts)).To(Succeed())
		Expect(cmRefPrompts.Data).To(HaveKey(pa.Name + ".prompt"))

		// inline-flow: prompt from inline content.
		var cmInlinePrompts corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-inline-flow-prompts"}, &cmInlinePrompts)).To(Succeed())
		Expect(cmInlinePrompts.Data).To(HaveKey("greeting.prompt"))
		Expect(cmInlinePrompts.Data["greeting.prompt"]).To(Equal(inlineContent))

		// inline-flow config: must reference inline model.
		var cmInlineCfg corev1.ConfigMap
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-inline-flow-config"}, &cmInlineCfg)).To(Succeed())
		Expect(cmInlineCfg.Data["config.json"]).To(ContainSubstring(`"claude-opus-4-7"`))
	})
})

// conditionStatus is a tiny helper for the tests above.
func conditionStatus(conds []metav1.Condition, t string) metav1.ConditionStatus {
	for _, c := range conds {
		if c.Type == t {
			return c.Status
		}
	}
	return metav1.ConditionUnknown
}
