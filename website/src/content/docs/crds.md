---
title: Custom Resources
description: Reference for every CRD the operator ships — what fields they have, how they reference each other.
---

All resources live in the `genkit.dev/v1alpha1` API group and are
namespaced.

## Reference graph

```
Flow / FlowSet ── pluginConfigRef ──▶ PluginConfig ── credentialsRef ──▶ Secret
   │                                       ▲
   ├── modelRef ──▶ Model ─────────────────┘
   ├── modelSpec (inline, pluginConfigRef still required)
   ├── promptRefs ──▶ Prompt(s)
   ├── prompts[].prompt (inline)
   └── toolRefs ────▶ Tool(s)
```

A `Flow` (or each entry in a `FlowSet`) ties one `Model` together with
one or more `Prompt`s and (optionally) `Tool`s. The `Model` points at a
`PluginConfig` which points at a `Secret`.

You can also skip the `Model` CR entirely by embedding the model
definition inline via `modelSpec`, and skip `Prompt` CRs by embedding
prompt content directly via `prompts[].prompt`. The `PluginConfig` CR is
still required when using `modelSpec` because it holds the credentials.

## PluginConfig

A provider definition. The `type` field selects a builder in the runner
(`anthropic`, `openai`, `googleai`, `vertexai`, `ollama`, `bedrock`, `azureaifoundry`).

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
  # extraConfig is opaque — passed through to the runner as JSON.
  # extraConfig:
  #   baseURL: "https://api.anthropic.com"
```

Short name: `gpc`.

## Model

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: claude-opus
spec:
  provider: anthropic
  model: claude-opus-4-6
  pluginConfigRef:
    name: anthropic
  info:
    label: "Anthropic — Claude Opus 4.6"
    supports:
      multiturn: true
      tools: true
      systemRole: true
  defaultConfig:
    temperature: 0.3
    maxOutputTokens: 1024
```

Short name: `gmd`.

## Prompt

A [Dotprompt](https://github.com/google/dotprompt) document — YAML
frontmatter followed by a Handlebars body.

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Prompt
metadata:
  name: greeting
spec:
  content: |
    ---
    model: anthropic/claude-opus-4-6
    temperature: 0.3
    ---
    Greet the user named {{name}} in a single sentence.
```

Short name: `gpr`.

## Tool

A `genkit-go` `ai.ToolDefinition` plus a dispatch target — either an
HTTP endpoint or a reference to a Flow.

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Tool
metadata:
  name: now
spec:
  definition:
    name: now
    description: "Return the current ISO-8601 timestamp."
    inputSchema:  { type: object, properties: {} }
    outputSchema: { type: string }
  implementation:
    http:
      url: "https://my-time-service.internal/now"
      method: POST
```

Short name: `gtl`.

## Flow

A single HTTP endpoint backed by a runner Pod.

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelRef: { name: claude-opus }
  promptRefs:
    - { name: greeting }
  toolRefs:
    - { name: now }
  port: 8080
  serviceType: ClusterIP
```

Exposed at `POST /<flow-name>` on the Pod's port (default `8080`).
Short name: `gfl`.

### Inline model spec

Instead of creating a `Model` CR and referencing it via `modelRef`, you
can embed the model definition directly in the `Flow` using `modelSpec`.
`modelRef` and `modelSpec` are **mutually exclusive** — use one or the
other.

```yaml
spec:
  modelSpec:
    provider: anthropic
    model: claude-opus-4-7
    pluginConfigRef:
      name: anthropic-config   # PluginConfig CR still required for credentials
    info:                      # optional
      label: "Anthropic — Claude Opus 4.7"
      supports:
        multiturn: true
        tools: true
        systemRole: true
    defaultConfig:             # optional
      temperature: 0.3
      maxOutputTokens: 1024
```

| Field             | Required | Description                                                  |
| ----------------- | -------- | ------------------------------------------------------------ |
| `provider`        | Yes      | Provider identifier (e.g. `anthropic`, `openai`, `googleai`) |
| `model`           | Yes      | Model name as recognised by the provider plugin              |
| `pluginConfigRef` | Yes      | Reference to a `PluginConfig` CR that holds the credentials  |
| `info`            | No       | Human-readable label and capability flags                    |
| `defaultConfig`   | No       | Default generation parameters (`temperature`, etc.)          |

### Inline prompts

Instead of creating `Prompt` CRs and referencing them via `promptRefs`,
you can embed prompt content directly in the `Flow` using the `prompts`
list with a `prompt` entry. `promptRef` and `prompt` are mutually
exclusive within a single list item.

```yaml
spec:
  prompts:
    - prompt:
        name: greeting         # becomes greeting.prompt on disk
        content: |
          ---
          model: anthropic/claude-opus-4-7
          ---
          Greet the user named {{name}} in a single sentence.
```

| Field     | Required | Description                                                        |
| --------- | -------- | ------------------------------------------------------------------ |
| `name`    | Yes      | Logical name; written as `<name>.prompt` in the runner's ConfigMap |
| `content` | Yes      | Full Dotprompt document (YAML frontmatter + Handlebars body)       |

You can mix styles — some items can use `promptRef` while others use
`prompt` inline — within the same `prompts` list.

## FlowSet

Multiple flows in one Pod. Each flow gets its own per-flow ConfigMaps
under `/genkit/flows/<flow-name>/` and its own credentials mount.

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
      promptRefs:
        - { name: greeting }
    - name: summarizer
      modelRef: { name: claude-opus }
      promptRefs:
        - { name: summarize }
```

Routes: `POST /greeter`, `POST /summarizer`. Short name: `gfs`.

### Inline model spec and prompts in FlowSet

Each flow entry inside `FlowSet.spec.flows` supports the same
`modelSpec` and `prompts[].prompt` inline fields as a standalone `Flow`.
This lets you define the entire FlowSet without creating any `Model` or
`Prompt` CRs.

```yaml
spec:
  flows:
    - name: greeter
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic-config
        defaultConfig:
          temperature: 0.3
      prompts:
        - prompt:
            name: greeting
            content: |
              ---
              model: anthropic/claude-opus-4-7
              ---
              Greet the user named {{name}} in a single sentence.
    - name: summarizer
      modelSpec:
        provider: anthropic
        model: claude-opus-4-7
        pluginConfigRef:
          name: anthropic-config
      prompts:
        - prompt:
            name: summarize
            content: |
              ---
              model: anthropic/claude-opus-4-7
              ---
              Summarize the following text in one paragraph: {{text}}
```

The same constraints apply as for `Flow`: `modelRef` and `modelSpec` are
mutually exclusive per flow entry, and `promptRef` / `prompt` are
mutually exclusive per list item.

## Dataset and Eval

Used together to run scheduled evaluations against a flow. See the
samples under `config/samples/`.

| Kind      | Short name | Purpose                                            |
| --------- | ---------- | -------------------------------------------------- |
| `Dataset` | `gds`      | A set of `{input, reference}` examples for evals   |
| `Eval`    | `gev`      | Scheduled run of a `Dataset` against a `Flow`      |

## Status conditions

Every CR carries the standard set:

- `Ready` — child resources reconciled successfully.
- `Reconciling` — the controller is currently working on it.
- `Degraded` — a reference is missing or rendering failed.

Inspect with `kubectl describe`.
