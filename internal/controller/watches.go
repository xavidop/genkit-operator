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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// secretToPluginConfigMapper returns an EventHandler that, given a Secret
// event, enqueues every PluginConfig in the same namespace that references
// that Secret in spec.credentialsRef.
func secretToPluginConfigMapper(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		var list genkitv1alpha1.PluginConfigList
		if err := c.List(ctx, &list, client.InNamespace(secret.Namespace)); err != nil {
			return nil
		}
		var out []reconcile.Request
		for _, pc := range list.Items {
			if pc.Spec.CredentialsRef.Name == secret.Name {
				out = append(out, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: pc.Namespace, Name: pc.Name},
				})
			}
		}
		return out
	})
}

// pluginConfigToModelMapper enqueues every Model that references the given
// PluginConfig.
func pluginConfigToModelMapper(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		pc, ok := obj.(*genkitv1alpha1.PluginConfig)
		if !ok {
			return nil
		}
		var list genkitv1alpha1.ModelList
		if err := c.List(ctx, &list, client.InNamespace(pc.Namespace)); err != nil {
			return nil
		}
		var out []reconcile.Request
		for _, m := range list.Items {
			if m.Spec.PluginConfigRef.Name == pc.Name {
				out = append(out, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: m.Namespace, Name: m.Name},
				})
			}
		}
		return out
	})
}

// flowDependencyMapper builds an EventHandler that enqueues every Flow in
// the same namespace whose spec references the given Prompt, Tool, or
// Model object by name. Pass nameSelector to pick a specific list ref.
func flowDependencyMapper(c client.Client, refKind string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetName()
		var flows genkitv1alpha1.FlowList
		if err := c.List(ctx, &flows, client.InNamespace(obj.GetNamespace())); err != nil {
			return nil
		}
		var out []reconcile.Request
		for _, f := range flows.Items {
			if flowReferences(&f, refKind, name) {
				out = append(out, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: f.Namespace, Name: f.Name},
				})
			}
		}
		return out
	})
}

// flowToToolMapper enqueues every Tool that wraps the given Flow.
func flowToToolMapper(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		flow, ok := obj.(*genkitv1alpha1.Flow)
		if !ok {
			return nil
		}
		var list genkitv1alpha1.ToolList
		if err := c.List(ctx, &list, client.InNamespace(flow.Namespace)); err != nil {
			return nil
		}
		var out []reconcile.Request
		for _, t := range list.Items {
			if t.Spec.Implementation.FlowRef != nil && t.Spec.Implementation.FlowRef.Name == flow.Name {
				out = append(out, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: t.Namespace, Name: t.Name},
				})
			}
		}
		return out
	})
}

// flowSetDependencyMapper enqueues every FlowSet in the same namespace
// whose spec references the given Prompt, Tool, or Model object by name.
func flowSetDependencyMapper(c client.Client, refKind string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetName()
		var list genkitv1alpha1.FlowSetList
		if err := c.List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
			return nil
		}
		var out []reconcile.Request
		for _, fs := range list.Items {
			if flowSetReferences(&fs, refKind, name) {
				out = append(out, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: fs.Namespace, Name: fs.Name},
				})
			}
		}
		return out
	})
}

func flowSetReferences(fs *genkitv1alpha1.FlowSet, kind, name string) bool {
	for _, f := range fs.Spec.Flows {
		switch kind {
		case "Prompt":
			for _, p := range f.Prompts {
				if p.Name == name {
					return true
				}
			}
		case "Tool":
			for _, t := range f.Tools {
				if t.Name == name {
					return true
				}
			}
		case "Model":
			if f.ModelRef.Name == name {
				return true
			}
		}
	}
	return false
}

func flowReferences(f *genkitv1alpha1.Flow, kind, name string) bool {
	switch kind {
	case "Prompt":
		for _, p := range f.Spec.Prompts {
			if p.Name == name {
				return true
			}
		}
	case "Tool":
		for _, t := range f.Spec.Tools {
			if t.Name == name {
				return true
			}
		}
	case "Model":
		if f.Spec.ModelRef != nil && f.Spec.ModelRef.Name == name {
			return true
		}
	}
	return false
}
