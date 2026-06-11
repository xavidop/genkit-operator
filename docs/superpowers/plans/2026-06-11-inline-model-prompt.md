# Inline Model Spec and Prompts for Flow/FlowSet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `modelSpec` (inline model config) and inline `prompt` entries to `Flow` and `FlowSet` CRDs so users can declare everything in one resource without creating separate `Model`/`Prompt` CRs.

**Architecture:** New shared types (`InlineModelSpec`, `InlinePrompt`, `PromptSource`) live in `api/v1alpha1/shared_types.go`. `FlowSpec` and `FlowSetFlow` gain a `modelSpec` field alongside the existing `modelRef`, and their `prompts` list entries change from bare `LocalObjectReference` to `PromptSource` (which wraps either `promptRef` or an inline `prompt`). Controllers resolve inline entries to the same internal `resolvedDeps` / `resolvedFlowSetFlow` structures — render functions are unchanged.

**Tech Stack:** Go 1.23, controller-runtime, kubebuilder markers, Ginkgo/Gomega tests, envtest.

---

### Task 1: Create `api/v1alpha1/shared_types.go`

**Files:**
- Create: `api/v1alpha1/shared_types.go`

- [ ] **Step 1: Create the file**

```go
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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// InlineModelSpec defines a model configuration inline within a Flow or
// FlowSet, as an alternative to referencing a Model CR via modelRef.
// Credentials are still supplied via a PluginConfig reference.
type InlineModelSpec struct {
	// Provider is the plugin type (e.g. "anthropic", "googleai").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Model is the provider model identifier (e.g. "claude-opus-4-7").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// PluginConfigRef references the PluginConfig that supplies credentials.
	// +kubebuilder:validation:Required
	PluginConfigRef corev1.LocalObjectReference `json:"pluginConfigRef"`

	// Info mirrors genkit-go ai.ModelInfo. Schemaless — passed as-is to the runtime.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Info *apiextensionsv1.JSON `json:"info,omitempty"`

	// DefaultConfig mirrors genkit-go ai.GenerationCommonConfig. Schemaless.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	DefaultConfig *apiextensionsv1.JSON `json:"defaultConfig,omitempty"`
}

// InlinePrompt is a named dotprompt document declared directly in a Flow or
// FlowSet without creating a Prompt CR. Name becomes the mounted filename
// (<name>.prompt); Content is the full dotprompt source.
type InlinePrompt struct {
	// Name is used as the prompt filename: <name>.prompt.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Content is the full dotprompt source (YAML frontmatter + Handlebars body).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`
}

// PromptSource is one entry in a Flow or FlowSet prompts list.
// Exactly one of promptRef or prompt must be set.
type PromptSource struct {
	// PromptRef references an existing Prompt CR by name.
	// +optional
	PromptRef *corev1.LocalObjectReference `json:"promptRef,omitempty"`

	// Prompt declares the prompt content inline (no Prompt CR required).
	// +optional
	Prompt *InlinePrompt `json:"prompt,omitempty"`
}

// GetName returns the prompt name from either the ref or the inline definition.
func (ps PromptSource) GetName() string {
	if ps.PromptRef != nil {
		return ps.PromptRef.Name
	}
	if ps.Prompt != nil {
		return ps.Prompt.Name
	}
	return ""
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./api/...
```

Expected: no output (clean build).

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha1/shared_types.go
git commit -m "feat(api): add InlineModelSpec, InlinePrompt, PromptSource shared types"
```

---

### Task 2: Update `FlowSpec` in `flow_types.go`

**Files:**
- Modify: `api/v1alpha1/flow_types.go:73-83`

- [ ] **Step 1: Update the Prompts and add ModelSpec fields**

In `api/v1alpha1/flow_types.go`, replace:

```go
	// Prompts is the list of Prompt CRs to mount under /genkit/prompts.
	// +optional
	Prompts []corev1.LocalObjectReference `json:"prompts,omitempty"`
```

with:

```go
	// Prompts is the list of prompts to mount under /genkit/prompts. Each
	// entry is either a promptRef (reference to a Prompt CR) or an inline
	// prompt declaration. Exactly one of the two fields must be set per entry.
	// +optional
	Prompts []PromptSource `json:"prompts,omitempty"`
```

And replace:

```go
	// ModelRef is the default Model for this Flow.
	// +optional
	ModelRef *corev1.LocalObjectReference `json:"modelRef,omitempty"`
```

with:

```go
	// ModelRef references an existing Model CR as the default model for this Flow.
	// Mutually exclusive with modelSpec; at most one may be set.
	// +optional
	ModelRef *corev1.LocalObjectReference `json:"modelRef,omitempty"`

	// ModelSpec declares the model configuration inline without creating a
	// Model CR. Mutually exclusive with modelRef; at most one may be set.
	// +optional
	ModelSpec *InlineModelSpec `json:"modelSpec,omitempty"`
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./api/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha1/flow_types.go
git commit -m "feat(api): add modelSpec and inline PromptSource to FlowSpec"
```

---

### Task 3: Update `FlowSetFlow` in `flowset_types.go`

**Files:**
- Modify: `api/v1alpha1/flowset_types.go:30-54`

- [ ] **Step 1: Update FlowSetFlow**

In `api/v1alpha1/flowset_types.go`, replace the entire `FlowSetFlow` struct body fields `Prompts`, `Tools`, and `ModelRef` with the updated version below. The struct comment and `Name` field stay unchanged:

```go
	// Prompts is the list of prompts available to this flow. The FIRST item
	// is the entrypoint invoked by POST /<name>; the rest are helpers loaded
	// into the same per-flow genkit registry. Each entry is either a
	// promptRef (reference to a Prompt CR) or an inline prompt declaration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Prompts []PromptSource `json:"prompts"`

	// Tools is the list of Tool CRs exposed to this flow.
	// +optional
	Tools []corev1.LocalObjectReference `json:"tools,omitempty"`

	// ModelRef references an existing Model CR as the default model for this
	// flow. Mutually exclusive with modelSpec; exactly one must be set.
	// +optional
	ModelRef *corev1.LocalObjectReference `json:"modelRef,omitempty"`

	// ModelSpec declares the model configuration inline without creating a
	// Model CR. Mutually exclusive with modelRef; exactly one must be set.
	// +optional
	ModelSpec *InlineModelSpec `json:"modelSpec,omitempty"`
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./api/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add api/v1alpha1/flowset_types.go
git commit -m "feat(api): add modelSpec and inline PromptSource to FlowSetFlow"
```

---

### Task 4: Update `flow_controller.go` — handle inline prompts and inline model

**Files:**
- Modify: `internal/controller/flow_controller.go:138-185`

- [ ] **Step 1: Replace the `resolveDeps` method body**

Replace the entire `resolveDeps` method (lines 138–186) with:

```go
func (r *FlowReconciler) resolveDeps(ctx context.Context, flow *genkitv1alpha1.Flow) (*resolvedDeps, error) {
	out := &resolvedDeps{}

	for _, ps := range flow.Spec.Prompts {
		switch {
		case ps.PromptRef != nil:
			var p genkitv1alpha1.Prompt
			if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: ps.PromptRef.Name}, &p); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("prompt %q not found", ps.PromptRef.Name)
				}
				return nil, err
			}
			out.prompts = append(out.prompts, p)
		case ps.Prompt != nil:
			out.prompts = append(out.prompts, genkitv1alpha1.Prompt{
				ObjectMeta: metav1.ObjectMeta{Name: ps.Prompt.Name},
				Spec:       genkitv1alpha1.PromptSpec{Content: ps.Prompt.Content},
			})
		default:
			return nil, fmt.Errorf("prompt source has neither promptRef nor prompt set")
		}
	}

	for _, ref := range flow.Spec.Tools {
		var t genkitv1alpha1.Tool
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: ref.Name}, &t); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("tool %q not found", ref.Name)
			}
			return nil, err
		}
		out.tools = append(out.tools, t)
	}

	switch {
	case flow.Spec.ModelRef != nil && flow.Spec.ModelSpec != nil:
		return nil, fmt.Errorf("modelRef and modelSpec are mutually exclusive")
	case flow.Spec.ModelRef != nil:
		var m genkitv1alpha1.Model
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: flow.Spec.ModelRef.Name}, &m); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("model %q not found", flow.Spec.ModelRef.Name)
			}
			return nil, err
		}
		out.model = &m
		var pc genkitv1alpha1.PluginConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: m.Spec.PluginConfigRef.Name}, &pc); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("pluginconfig %q not found (referenced by model %q)", m.Spec.PluginConfigRef.Name, m.Name)
			}
			return nil, err
		}
		out.plugin = &pc
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("secret %q not found (referenced by pluginconfig %q)", pc.Spec.CredentialsRef.Name, pc.Name)
			}
			return nil, err
		}
		out.secret = &secret
	case flow.Spec.ModelSpec != nil:
		out.model = &genkitv1alpha1.Model{
			Spec: genkitv1alpha1.ModelSpec{
				Provider:        flow.Spec.ModelSpec.Provider,
				Model:           flow.Spec.ModelSpec.Model,
				PluginConfigRef: flow.Spec.ModelSpec.PluginConfigRef,
				Info:            flow.Spec.ModelSpec.Info,
				DefaultConfig:   flow.Spec.ModelSpec.DefaultConfig,
			},
		}
		var pc genkitv1alpha1.PluginConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: flow.Spec.ModelSpec.PluginConfigRef.Name}, &pc); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("pluginconfig %q not found (referenced by inline modelSpec)", flow.Spec.ModelSpec.PluginConfigRef.Name)
			}
			return nil, err
		}
		out.plugin = &pc
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Namespace: flow.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("secret %q not found (referenced by pluginconfig %q)", pc.Spec.CredentialsRef.Name, pc.Name)
			}
			return nil, err
		}
		out.secret = &secret
	}

	return out, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/controller/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/flow_controller.go
git commit -m "feat(controller): resolve inline modelSpec and inline prompts in Flow"
```

---

### Task 5: Update `flowset_controller.go` — handle inline prompts and inline model

**Files:**
- Modify: `internal/controller/flowset_controller.go:158-224`
- Modify: `internal/controller/flowset_render.go:164-167` (entrypoint extraction)

- [ ] **Step 1: Fix entrypoint extraction in `flowset_render.go`**

In `internal/controller/flowset_render.go`, replace:

```go
		entrypoint := ""
		if len(rf.template.Prompts) > 0 {
			entrypoint = rf.template.Prompts[0].Name
		}
```

with:

```go
		entrypoint := ""
		if len(rf.template.Prompts) > 0 {
			entrypoint = rf.template.Prompts[0].GetName()
		}
```

- [ ] **Step 2: Replace the `resolveDeps` method in `flowset_controller.go`**

Replace the entire `resolveDeps` method (lines 158–225) with:

```go
func (r *FlowSetReconciler) resolveDeps(ctx context.Context, fs *genkitv1alpha1.FlowSet) (*resolvedFlowSet, error) {
	out := &resolvedFlowSet{flows: make([]resolvedFlowSetFlow, 0, len(fs.Spec.Flows))}
	seen := map[string]struct{}{}

	for _, tmpl := range fs.Spec.Flows {
		if _, dup := seen[tmpl.Name]; dup {
			return nil, fmt.Errorf("duplicate flow name %q", tmpl.Name)
		}
		seen[tmpl.Name] = struct{}{}

		rf := resolvedFlowSetFlow{template: tmpl}

		for _, ps := range tmpl.Prompts {
			switch {
			case ps.PromptRef != nil:
				var p genkitv1alpha1.Prompt
				if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: ps.PromptRef.Name}, &p); err != nil {
					if apierrors.IsNotFound(err) {
						return nil, fmt.Errorf("flow %q: prompt %q not found", tmpl.Name, ps.PromptRef.Name)
					}
					return nil, err
				}
				rf.prompts = append(rf.prompts, p)
			case ps.Prompt != nil:
				rf.prompts = append(rf.prompts, genkitv1alpha1.Prompt{
					ObjectMeta: metav1.ObjectMeta{Name: ps.Prompt.Name},
					Spec:       genkitv1alpha1.PromptSpec{Content: ps.Prompt.Content},
				})
			default:
				return nil, fmt.Errorf("flow %q: prompt source has neither promptRef nor prompt set", tmpl.Name)
			}
		}

		for _, ref := range tmpl.Tools {
			var t genkitv1alpha1.Tool
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: ref.Name}, &t); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: tool %q not found", tmpl.Name, ref.Name)
				}
				return nil, err
			}
			rf.tools = append(rf.tools, t)
		}

		switch {
		case tmpl.ModelRef != nil && tmpl.ModelSpec != nil:
			return nil, fmt.Errorf("flow %q: modelRef and modelSpec are mutually exclusive", tmpl.Name)
		case tmpl.ModelRef == nil && tmpl.ModelSpec == nil:
			return nil, fmt.Errorf("flow %q: exactly one of modelRef or modelSpec is required", tmpl.Name)
		case tmpl.ModelRef != nil:
			var m genkitv1alpha1.Model
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: tmpl.ModelRef.Name}, &m); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: model %q not found", tmpl.Name, tmpl.ModelRef.Name)
				}
				return nil, err
			}
			rf.model = m
			var pc genkitv1alpha1.PluginConfig
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: m.Spec.PluginConfigRef.Name}, &pc); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: pluginconfig %q not found (referenced by model %q)", tmpl.Name, m.Spec.PluginConfigRef.Name, m.Name)
				}
				return nil, err
			}
			rf.plugin = pc
			var secret corev1.Secret
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: secret %q not found (referenced by pluginconfig %q)", tmpl.Name, pc.Spec.CredentialsRef.Name, pc.Name)
				}
				return nil, err
			}
			rf.secretName = secret.Name
			rf.secret = &secret
		case tmpl.ModelSpec != nil:
			rf.model = genkitv1alpha1.Model{
				Spec: genkitv1alpha1.ModelSpec{
					Provider:        tmpl.ModelSpec.Provider,
					Model:           tmpl.ModelSpec.Model,
					PluginConfigRef: tmpl.ModelSpec.PluginConfigRef,
					Info:            tmpl.ModelSpec.Info,
					DefaultConfig:   tmpl.ModelSpec.DefaultConfig,
				},
			}
			var pc genkitv1alpha1.PluginConfig
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: tmpl.ModelSpec.PluginConfigRef.Name}, &pc); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: pluginconfig %q not found (referenced by inline modelSpec)", tmpl.Name, tmpl.ModelSpec.PluginConfigRef.Name)
				}
				return nil, err
			}
			rf.plugin = pc
			var secret corev1.Secret
			if err := r.Get(ctx, types.NamespacedName{Namespace: fs.Namespace, Name: pc.Spec.CredentialsRef.Name}, &secret); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("flow %q: secret %q not found (referenced by pluginconfig %q)", tmpl.Name, pc.Spec.CredentialsRef.Name, pc.Name)
				}
				return nil, err
			}
			rf.secretName = secret.Name
			rf.secret = &secret
		}

		out.flows = append(out.flows, rf)
	}

	return out, nil
}
```

- [ ] **Step 3: Verify everything compiles**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/flowset_controller.go internal/controller/flowset_render.go
git commit -m "feat(controller): resolve inline modelSpec and inline prompts in FlowSet"
```

---

### Task 6: Regenerate CRDs and DeepCopy methods

**Files:**
- Modify: `config/crd/bases/genkit.dev_flows.yaml` (regenerated)
- Modify: `config/crd/bases/genkit.dev_flowsets.yaml` (regenerated)
- Modify: `api/v1alpha1/zz_generated.deepcopy.go` (regenerated)

- [ ] **Step 1: Run code and manifest generation**

```bash
make manifests generate
```

Expected: no errors. The three files above will be updated by controller-gen.

- [ ] **Step 2: Commit the generated files**

```bash
git add config/crd/bases/genkit.dev_flows.yaml config/crd/bases/genkit.dev_flowsets.yaml api/v1alpha1/zz_generated.deepcopy.go
git commit -m "chore: regenerate CRDs and DeepCopy for InlineModelSpec/PromptSource types"
```

---

### Task 7: Fix existing tests to compile

The `Prompts` type changed from `[]corev1.LocalObjectReference` to `[]PromptSource`, and `FlowSetFlow.ModelRef` changed from `corev1.LocalObjectReference` (value) to `*corev1.LocalObjectReference` (pointer). Existing tests reference these types directly and will fail to compile.

**Files:**
- Modify: `internal/controller/flow_controller_test.go`
- Modify: `internal/controller/flowset_controller_test.go`

- [ ] **Step 1: Update `flow_controller_test.go`**

There are four occurrences of `Prompts: []corev1.LocalObjectReference{{Name: ...}}` in this file. Replace ALL of them with `Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: ...}}}`.

Occurrence 1 (happy path test, ~line 88):
```go
// Before:
Prompts:  []corev1.LocalObjectReference{{Name: p.Name}},
// After:
Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: p.Name}}},
```

Occurrence 2 (missing prompt test, ~line 116):
```go
// Before:
Prompts: []corev1.LocalObjectReference{{Name: "nope"}},
// After:
Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: "nope"}}},
```

Occurrence 3 (hash change test, ~line 136):
```go
// Before:
Prompts: []corev1.LocalObjectReference{{Name: p.Name}},
// After:
Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: p.Name}}},
```

Occurrence 4 (secret hash test, ~line 175):
```go
// Before:
Prompts:  []corev1.LocalObjectReference{{Name: p.Name}},
// After:
Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: p.Name}}},
```

- [ ] **Step 2: Update `flowset_controller_test.go`**

Three test cases use `ModelRef: corev1.LocalObjectReference{Name: ...}` (value) — change to pointer. Same tests also use `Prompts: []corev1.LocalObjectReference{{Name: ...}}` — change to `PromptSource`.

Occurrence 1 (happy path test `fs-ok`, ~lines 49-59):
```go
// Before:
{
    Name:     "alpha",
    ModelRef: corev1.LocalObjectReference{Name: m.Name},
    Prompts:  []corev1.LocalObjectReference{{Name: pa.Name}},
},
{
    Name:     "beta",
    ModelRef: corev1.LocalObjectReference{Name: m.Name},
    Prompts:  []corev1.LocalObjectReference{{Name: pb.Name}},
},
// After:
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
```

Occurrence 2 (missing model test `fs-nomodel`, ~lines 108-113):
```go
// Before:
{
    Name:     "lone",
    ModelRef: corev1.LocalObjectReference{Name: "nope"},
    Prompts:  []corev1.LocalObjectReference{{Name: p.Name}},
},
// After:
{
    Name:     "lone",
    ModelRef: &corev1.LocalObjectReference{Name: "nope"},
    Prompts:  []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: p.Name}}},
},
```

Occurrence 3 (gc orphans test `fs-gc`, ~lines 141-144):
```go
// Before:
{Name: "alpha", ModelRef: corev1.LocalObjectReference{Name: m.Name},
    Prompts: []corev1.LocalObjectReference{{Name: pa.Name}}},
{Name: "beta", ModelRef: corev1.LocalObjectReference{Name: m.Name},
    Prompts: []corev1.LocalObjectReference{{Name: pb.Name}}},
// After:
{Name: "alpha", ModelRef: &corev1.LocalObjectReference{Name: m.Name},
    Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pa.Name}}}},
{Name: "beta", ModelRef: &corev1.LocalObjectReference{Name: m.Name},
    Prompts: []genkitv1alpha1.PromptSource{{PromptRef: &corev1.LocalObjectReference{Name: pb.Name}}}},
```

- [ ] **Step 3: Verify tests compile**

```bash
go build ./internal/controller/...
```

Expected: no output.

- [ ] **Step 4: Run existing tests to confirm they still pass**

```bash
make test
```

Expected: all tests pass, `ok github.com/xavidop/genkit-operator/internal/controller`.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/flow_controller_test.go internal/controller/flowset_controller_test.go
git commit -m "test: update existing tests for PromptSource and ModelRef pointer type changes"
```

---

### Task 8: Add new inline tests for Flow

**Files:**
- Modify: `internal/controller/flow_controller_test.go`

- [ ] **Step 1: Add two new test cases inside `Describe("Flow Controller", ...)`**

Append these two `It` blocks before the closing `})` of the `Describe("Flow Controller", ...)` block:

```go
It("renders config.json using inline modelSpec (no Model CR needed)", func() {
    name := uniqueName("flow-inlinemodel")
    pc := makeReadyPluginConfig("default", name+"-pc")
    p := makeReadyPrompt(name+"-prompt", "---\nmodel: x\n---\nhi")

    flow := &genkitv1alpha1.Flow{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
        Spec: genkitv1alpha1.FlowSpec{
            Image: "ghcr.io/example/flow:1",
            ModelSpec: &genkitv1alpha1.InlineModelSpec{
                Provider:        "anthropic",
                Model:           "claude-opus-4-7",
                PluginConfigRef: corev1.LocalObjectReference{Name: pc.Name},
            },
            Prompts: []genkitv1alpha1.PromptSource{
                {PromptRef: &corev1.LocalObjectReference{Name: p.Name}},
            },
        },
    }
    Expect(k8sClient.Create(ctx, flow)).To(Succeed())
    DeferCleanup(func() { _ = k8sClient.Delete(ctx, flow) })

    r := &FlowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
    _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
    Expect(err).NotTo(HaveOccurred())

    var cfgCM corev1.ConfigMap
    Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-config"}, &cfgCM)).To(Succeed())
    Expect(cfgCM.Data["config.json"]).To(ContainSubstring(`"provider": "anthropic"`))
    Expect(cfgCM.Data["config.json"]).To(ContainSubstring(`"model": "claude-opus-4-7"`))
})

It("mounts inline prompt content without a Prompt CR", func() {
    name := uniqueName("flow-inlineprompt")
    m := makeReadyModel(name + "-model")

    flow := &genkitv1alpha1.Flow{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
        Spec: genkitv1alpha1.FlowSpec{
            Image:    "ghcr.io/example/flow:1",
            ModelRef: &corev1.LocalObjectReference{Name: m.Name},
            Prompts: []genkitv1alpha1.PromptSource{
                {Prompt: &genkitv1alpha1.InlinePrompt{
                    Name:    "my-greeting",
                    Content: "---\nmodel: anthropic/claude-opus-4-7\n---\nHello {{name}}",
                }},
            },
        },
    }
    Expect(k8sClient.Create(ctx, flow)).To(Succeed())
    DeferCleanup(func() { _ = k8sClient.Delete(ctx, flow) })

    r := &FlowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
    _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
    Expect(err).NotTo(HaveOccurred())

    var promptsCM corev1.ConfigMap
    Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-prompts"}, &promptsCM)).To(Succeed())
    Expect(promptsCM.Data).To(HaveKey("my-greeting.prompt"))
    Expect(promptsCM.Data["my-greeting.prompt"]).To(ContainSubstring("Hello {{name}}"))
})
```

- [ ] **Step 2: Run the new tests**

```bash
make test
```

Expected: all tests pass including the two new ones.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/flow_controller_test.go
git commit -m "test: add inline modelSpec and inline prompt tests for Flow controller"
```

---

### Task 9: Add new inline tests for FlowSet

**Files:**
- Modify: `internal/controller/flowset_controller_test.go`

- [ ] **Step 1: Add one new test case inside `Describe("FlowSet Controller", ...)`**

Append this `It` block before the closing `})` of the `Describe("FlowSet Controller", ...)` block:

```go
It("renders FlowSet with inline modelSpec and inline prompt (no Model or Prompt CR)", func() {
    name := uniqueName("fs-inline")
    pc := makeReadyPluginConfig("default", name+"-pc")

    fs := &genkitv1alpha1.FlowSet{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
        Spec: genkitv1alpha1.FlowSetSpec{
            Image: "ghcr.io/example/runner:1",
            Flows: []genkitv1alpha1.FlowSetFlow{
                {
                    Name: "inline-flow",
                    ModelSpec: &genkitv1alpha1.InlineModelSpec{
                        Provider:        "anthropic",
                        Model:           "claude-opus-4-7",
                        PluginConfigRef: corev1.LocalObjectReference{Name: pc.Name},
                    },
                    Prompts: []genkitv1alpha1.PromptSource{
                        {Prompt: &genkitv1alpha1.InlinePrompt{
                            Name:    "greeting",
                            Content: "---\nmodel: anthropic/claude-opus-4-7\n---\nHi {{name}}",
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

    // Prompt content must appear under the flow's prompts CM.
    var promptsCM corev1.ConfigMap
    Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-inline-flow-prompts"}, &promptsCM)).To(Succeed())
    Expect(promptsCM.Data).To(HaveKey("greeting.prompt"))
    Expect(promptsCM.Data["greeting.prompt"]).To(ContainSubstring("Hi {{name}}"))

    // config.json must reflect the inline model.
    var cfgCM corev1.ConfigMap
    Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-inline-flow-config"}, &cfgCM)).To(Succeed())
    Expect(cfgCM.Data["config.json"]).To(ContainSubstring(`"provider": "anthropic"`))
    Expect(cfgCM.Data["config.json"]).To(ContainSubstring(`"model": "claude-opus-4-7"`))

    // Manifest must use the inline prompt name as entrypoint.
    var manifestCM corev1.ConfigMap
    Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-manifest"}, &manifestCM)).To(Succeed())
    Expect(manifestCM.Data["manifest.json"]).To(ContainSubstring(`"entrypoint": "greeting"`))
})
```

- [ ] **Step 2: Run all tests**

```bash
make test
```

Expected: all tests pass including the new FlowSet inline test.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/flowset_controller_test.go
git commit -m "test: add inline modelSpec and inline prompt test for FlowSet controller"
```

---

### Task 10: Update `config/samples/`

**Files:**
- Modify: `config/samples/genkit_v1alpha1_flow.yaml`
- Modify: `config/samples/genkit_v1alpha1_flowset.yaml`

- [ ] **Step 1: Update `config/samples/genkit_v1alpha1_flow.yaml`**

Replace the entire file contents with a sample that shows BOTH styles side by side:

```yaml
# Example A: ref-based (existing style, unchanged)
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter-refs
  namespace: default
spec:
  image: ghcr.io/xavidop/genkit-runner:latest
  replicas: 1
  port: 8080
  modelRef:
    name: claude-opus-47
  prompts:
    - promptRef:
        name: greeting
  tools:
    - name: weather-lookup
  serviceType: ClusterIP
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
---
# Example B: inline style (no Model or Prompt CR needed)
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter-inline
  namespace: default
spec:
  image: ghcr.io/xavidop/genkit-runner:latest
  replicas: 1
  port: 8080
  modelSpec:
    provider: anthropic
    model: claude-opus-4-7
    pluginConfigRef:
      name: anthropic
    defaultConfig:
      temperature: 0.3
      maxOutputTokens: 1024
  prompts:
    - prompt:
        name: greeting
        content: |
          ---
          model: anthropic/claude-opus-4-7
          config:
            max_tokens: 256
          input:
            schema:
              name: string
          ---
          You are a friendly assistant. Greet {{name}} in one short sentence.
  serviceType: ClusterIP
```

- [ ] **Step 2: Update `config/samples/genkit_v1alpha1_flowset.yaml`**

Replace the entire file contents:

```yaml
# Example A: ref-based (existing style, unchanged)
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: greeting-suite-refs
  namespace: default
spec:
  image: ghcr.io/xavidop/genkit-runner:latest
  imagePullPolicy: IfNotPresent
  replicas: 1
  port: 8080
  serviceType: ClusterIP
  flows:
    - name: greeter-en
      modelRef:
        name: claude-opus-47
      prompts:
        - promptRef:
            name: greeting
    - name: greeter-es
      modelRef:
        name: claude-opus-47
      prompts:
        - promptRef:
            name: greeting
---
# Example B: inline style (no Model or Prompt CRs needed per flow)
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: greeting-suite-inline
  namespace: default
spec:
  image: ghcr.io/xavidop/genkit-runner:latest
  imagePullPolicy: IfNotPresent
  replicas: 1
  port: 8080
  serviceType: ClusterIP
  flows:
    - name: greeter-en
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic
      prompts:
        - prompt:
            name: greeting-en
            content: |
              ---
              model: anthropic/claude-opus-4-7
              ---
              Greet {{name}} in English in one sentence.
    - name: greeter-es
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic
      prompts:
        - prompt:
            name: greeting-es
            content: |
              ---
              model: anthropic/claude-opus-4-7
              ---
              Saluda a {{name}} en español en una frase corta.
```

- [ ] **Step 3: Commit**

```bash
git add config/samples/genkit_v1alpha1_flow.yaml config/samples/genkit_v1alpha1_flowset.yaml
git commit -m "docs(samples): show both ref-based and inline styles for Flow and FlowSet"
```

---

### Task 11: Update website docs

**Files:**
- Modify: `website/src/content/docs/crds.md`
- Modify: `website/src/content/docs/guides/deploy-flow.md`
- Modify: `website/src/content/docs/guides/flowset.md`

- [ ] **Step 1: Update `website/src/content/docs/crds.md` — Flow section**

In the `## Flow` section, replace the existing YAML example and paragraph with:

```markdown
## Flow

A single HTTP endpoint backed by a runner Pod.

**Option A — ref-based (references existing `Model` and `Prompt` CRs):**

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelRef: { name: claude-opus }
  prompts:
    - promptRef: { name: greeting }
  port: 8080
  serviceType: ClusterIP
```

**Option B — inline (no `Model` or `Prompt` CRs needed; still requires a `PluginConfig`):**

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelSpec:
    provider: anthropic
    model: claude-opus-4-7
    pluginConfigRef:
      name: anthropic
    defaultConfig:
      temperature: 0.3
  prompts:
    - prompt:
        name: greeting
        content: |
          ---
          model: anthropic/claude-opus-4-7
          ---
          Greet {{name}} in one sentence.
  port: 8080
  serviceType: ClusterIP
```

`modelRef` and `modelSpec` are mutually exclusive; set exactly one. Each
`prompts` entry uses either `promptRef` (CR lookup) or `prompt` (inline).
Exposed at `POST /<flow-name>` on the Pod's port (default `8080`).
Short name: `gfl`.
```

- [ ] **Step 2: Update `website/src/content/docs/crds.md` — FlowSet section**

In the `## FlowSet` section, replace the existing YAML example with:

```markdown
## FlowSet

Multiple flows in one Pod. Each flow gets its own per-flow ConfigMaps
under `/genkit/flows/<flow-name>/` and its own credentials mount.

Per-flow entries support the same `modelRef`/`modelSpec` and `promptRef`/`prompt`
options as `Flow`.

**Option A — ref-based:**

```yaml
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: assistants
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  port: 8080
  flows:
    - name: greeter
      modelRef: { name: claude-opus }
      prompts:
        - promptRef: { name: greeting }
    - name: summarizer
      modelRef: { name: claude-opus }
      prompts:
        - promptRef: { name: summarize }
```

**Option B — inline:**

```yaml
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: assistants
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  port: 8080
  flows:
    - name: greeter
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic
      prompts:
        - prompt:
            name: greeting
            content: |
              Greet {{name}} in one sentence.
    - name: summarizer
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic
      prompts:
        - prompt:
            name: summarize
            content: |
              Summarize the following in three bullets: {{text}}
```

Routes: `POST /greeter`, `POST /summarizer`. Short name: `gfs`.
```

- [ ] **Step 3: Update `website/src/content/docs/guides/deploy-flow.md` — add inline section**

At the end of the file (after "## 7. Iterate"), add:

```markdown
## Alternative: inline model and prompt

If you'd rather not create separate `Model` and `Prompt` CRs, declare
everything inline in the `Flow` spec. You still need the `PluginConfig`
and its `Secret` for credentials.

```bash
kubectl create secret generic anthropic-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

```yaml
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: anthropic
spec:
  type: anthropic
  credentialsRef:
    name: anthropic-credentials
  credentialKeys: [ANTHROPIC_API_KEY]
---
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelSpec:
    provider: anthropic
    model: claude-opus-4-7
    pluginConfigRef:
      name: anthropic
    defaultConfig:
      temperature: 0.3
      maxOutputTokens: 1024
  prompts:
    - prompt:
        name: greeting
        content: |
          ---
          model: anthropic/claude-opus-4-7
          config:
            max_tokens: 256
          input:
            schema:
              name: string
          ---
          You are a friendly assistant. Greet {{name}} in one short sentence.
  port: 8080
```

`modelRef` and `modelSpec` are mutually exclusive. Use `modelRef` when you
want to share a model definition across flows; use `modelSpec` for
self-contained single-file deployments.
```

- [ ] **Step 4: Update `website/src/content/docs/guides/flowset.md` — add inline section**

After the `## Example` section, add a new section:

```markdown
## Inline model and prompt (no Model or Prompt CRs)

Each flow entry in a `FlowSet` also supports the `modelSpec` and inline
`prompt` options. You still need a `PluginConfig` and its `Secret`.

```yaml
flows:
  - name: greeter-en
    modelSpec:
      provider: anthropic
      model: claude-opus-4-7
      pluginConfigRef:
        name: anthropic
    prompts:
      - prompt:
          name: greeting-en
          content: |
            Greet {{name}} in English in one sentence.
  - name: greeter-es
    modelSpec:
      provider: anthropic
      model: claude-opus-4-7
      pluginConfigRef:
        name: anthropic
    prompts:
      - prompt:
          name: greeting-es
          content: |
            Saluda a {{name}} en español en una frase corta.
```

`modelRef` and `modelSpec` are mutually exclusive per flow entry.
```

- [ ] **Step 5: Commit**

```bash
git add website/src/content/docs/crds.md website/src/content/docs/guides/deploy-flow.md website/src/content/docs/guides/flowset.md
git commit -m "docs: document inline modelSpec and prompt options for Flow and FlowSet"
```
