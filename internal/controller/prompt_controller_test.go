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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

var _ = Describe("Prompt Controller", func() {
	const ns = "default"

	It("computes content hash and parses frontmatter", func() {
		name := uniqueName("prompt-ok")
		content := "---\nmodel: anthropic/claude-opus-4-7\ntemperature: 0.3\n---\nHello {{name}}!"
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
		Expect(readyStatus(got.Status.Conditions)).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.ContentHash).NotTo(BeEmpty())
		Expect(got.Status.ParsedFrontmatter).NotTo(BeNil())
	})
})
