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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

func makeFlow(ns, name string) *genkitv1alpha1.Flow {
	f := &genkitv1alpha1.Flow{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       genkitv1alpha1.FlowSpec{Image: "ghcr.io/example/flow:1"},
	}
	Expect(k8sClient.Create(ctx, f)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, f) })
	return f
}

func makeReadyDataset(ns, name string) *genkitv1alpha1.Dataset {
	d := &genkitv1alpha1.Dataset{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: genkitv1alpha1.DatasetSpec{
			Format: genkitv1alpha1.DatasetFormatJSON,
			Source: genkitv1alpha1.DatasetSource{
				Inline: &apiextensionsv1.JSON{Raw: []byte(`[{"x":1}]`)},
			},
		},
	}
	Expect(k8sClient.Create(ctx, d)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, d) })
	return d
}

var _ = Describe("Eval Controller", func() {
	const ns = "default"

	It("creates a one-shot Job when no schedule is set", func() {
		name := uniqueName("eval-job")
		flow := makeFlow(ns, name+"-flow")
		ds := makeReadyDataset(ns, name+"-ds")

		e := &genkitv1alpha1.Eval{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.EvalSpec{
				FlowRef:     corev1.LocalObjectReference{Name: flow.Name},
				DatasetRef:  corev1.LocalObjectReference{Name: ds.Name},
				Metrics:     []string{"faithfulness"},
				RunnerImage: "ghcr.io/example/eval-runner:1",
				OutputSink: genkitv1alpha1.EvalOutputSink{
					ConfigMap: &genkitv1alpha1.ConfigMapSink{Name: name + "-out", Key: "result.json"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, e)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, e) })

		r := &EvalReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		var job batchv1.Job
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: e.Name + "-run"}, &job)).To(Succeed())
		Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(e.Spec.RunnerImage))
	})

	It("creates a CronJob when schedule is set", func() {
		name := uniqueName("eval-cron")
		flow := makeFlow(ns, name+"-flow")
		ds := makeReadyDataset(ns, name+"-ds")

		e := &genkitv1alpha1.Eval{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: genkitv1alpha1.EvalSpec{
				FlowRef:     corev1.LocalObjectReference{Name: flow.Name},
				DatasetRef:  corev1.LocalObjectReference{Name: ds.Name},
				Metrics:     []string{"correctness"},
				Schedule:    "*/5 * * * *",
				RunnerImage: "ghcr.io/example/eval-runner:1",
				OutputSink: genkitv1alpha1.EvalOutputSink{
					ConfigMap: &genkitv1alpha1.ConfigMapSink{Name: name + "-out", Key: "result.json"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, e)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, e) })

		r := &EvalReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())

		var cj batchv1.CronJob
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: e.Name}, &cj)).To(Succeed())
		Expect(cj.Spec.Schedule).To(Equal("*/5 * * * *"))
	})
})
