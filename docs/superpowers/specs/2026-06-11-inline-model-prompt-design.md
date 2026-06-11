# Inline Model Spec and Prompts for Flow and FlowSet

**Date:** 2026-06-11
**Status:** Approved

## Problem

Users who want to build a Flow or FlowSet without creating separate `Model` and `Prompt` CRs have no way to do so today. Every flow requires a `modelRef` pointing to a `Model` CR and a `prompts` list pointing to `Prompt` CRs. This friction is unnecessary for quick iterations or simple deployments where the full CRD graph is overkill.

## Goal

Add inline alternatives for model spec and prompts directly in `Flow` and `FlowSet`, while keeping all existing ref-based fields fully intact and unchanged.

## Migration Note

The `prompts` field element type changes from `corev1.LocalObjectReference` (bare `name`) to `PromptSource`. Existing YAML using `- name: greeting` must be updated to `- promptRef: {name: greeting}`. Since this is v1alpha1, this breaking change is acceptable.

## Out of Scope

- Inlining `PluginConfig` credentials — a `PluginConfig` CR is still required for credentials even when using an inline model spec.
- Inlining `Tool` definitions.
- Deprecating or removing `modelRef` or existing prompt refs.

---

## API Design

### New Types (api/v1alpha1)

#### `InlineModelSpec`

Mirrors `ModelSpec` from the `Model` CR, but declared directly in the Flow. The `PluginConfig` ref for credentials is still required.

```go
type InlineModelSpec struct {
    Provider        string                        `json:"provider"`
    Model           string                        `json:"model"`
    PluginConfigRef corev1.LocalObjectReference   `json:"pluginConfigRef"`
    Info            *apiextensionsv1.JSON         `json:"info,omitempty"`
    DefaultConfig   *apiextensionsv1.JSON         `json:"defaultConfig,omitempty"`
}
```

#### `InlinePrompt`

A named dotprompt document declared inline. `Name` becomes the mounted filename (`<name>.prompt`). `Content` is the full dotprompt source (YAML frontmatter + Handlebars body).

```go
type InlinePrompt struct {
    Name    string `json:"name"`
    Content string `json:"content"`
}
```

#### `PromptSource`

Each entry in the `prompts` list is either a ref to a `Prompt` CR or an inline prompt. Exactly one of the two fields must be set.

```go
type PromptSource struct {
    PromptRef *corev1.LocalObjectReference `json:"promptRef,omitempty"`
    Prompt    *InlinePrompt                `json:"prompt,omitempty"`
}
```

---

### `FlowSpec` Changes

`modelRef` is now a pointer (optional). `modelSpec` is a new optional field. Exactly one must be set.
`prompts` changes element type from `corev1.LocalObjectReference` to `PromptSource`.

```go
// Before
ModelRef corev1.LocalObjectReference     `json:"modelRef"`
Prompts  []corev1.LocalObjectReference   `json:"prompts,omitempty"`

// After
ModelRef  *corev1.LocalObjectReference   `json:"modelRef,omitempty"`
ModelSpec *InlineModelSpec               `json:"modelSpec,omitempty"`
Prompts   []PromptSource                 `json:"prompts,omitempty"`
```

### `FlowSetFlow` Changes

Same treatment — `modelRef` becomes a pointer, `modelSpec` added, `prompts` updated to `[]PromptSource`.

---

## Validation Rules

| Field | Rule |
|---|---|
| `modelRef` / `modelSpec` | Exactly one must be set. Controller returns error if both or neither are set. |
| `PromptSource` | Exactly one of `promptRef` / `prompt` must be set per entry. |
| `InlinePrompt.Name` | Required, non-empty. |
| `InlinePrompt.Content` | Required, non-empty. |
| `InlineModelSpec.Provider` | Required, non-empty. |
| `InlineModelSpec.Model` | Required, non-empty. |
| `InlineModelSpec.PluginConfigRef` | Required, must reference an existing PluginConfig. |

Validation is enforced in the controller reconcile loop (not webhook, to keep the operator lightweight).

---

## Controller Behavior

### flow_render.go

When building `config.json` for the Flow ConfigMap:
1. If `spec.modelSpec` is set → use its fields directly to construct the model config block.
2. If `spec.modelRef` is set → look up the `Model` CR by name (existing behavior).

When building the prompts ConfigMap:
1. For each `PromptSource` in `spec.prompts`:
   - If `promptRef` is set → look up the `Prompt` CR by name, use its `spec.content` (existing behavior).
   - If `prompt` is set → use `prompt.content` directly; mount as `<prompt.name>.prompt`.

### flowset_render.go

Same logic applied per-flow inside the FlowSet flow entries.

---

## Example YAML

### Flow with inline model and inline prompt

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
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

### Flow with refs (existing behavior, unchanged)

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
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
  serviceType: ClusterIP
```

### FlowSet mixing refs and inline

```yaml
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: greeting-suite
  namespace: default
spec:
  image: ghcr.io/xavidop/genkit-runner:latest
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
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic
      prompts:
        - prompt:
            name: saludo
            content: |
              ---
              model: anthropic/claude-opus-4-7
              ---
              Saluda a {{name}} en español en una frase corta.
```

---

## Files to Change

| File | Change |
|---|---|
| `api/v1alpha1/flow_types.go` | Add `InlineModelSpec`, `InlinePrompt`, `PromptSource`; update `FlowSpec` |
| `api/v1alpha1/flowset_types.go` | Update `FlowSetFlow` with same new types |
| `internal/controller/flow_render.go` | Handle inline model and inline prompt rendering |
| `internal/controller/flowset_render.go` | Same for FlowSet |
| `config/crd/bases/genkit.dev_flows.yaml` | Regenerated via `make manifests` |
| `config/crd/bases/genkit.dev_flowsets.yaml` | Regenerated via `make manifests` |
| `config/samples/genkit_v1alpha1_flow.yaml` | Updated to show inline usage |
| `config/samples/genkit_v1alpha1_flowset.yaml` | Updated to show inline usage |
| `website/src/content/docs/crds.md` | Document new fields and examples |
| `website/src/content/docs/guides/deploy-flow.md` | Add inline section |
| `website/src/content/docs/guides/flowset.md` | Add inline section |
