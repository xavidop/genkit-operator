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
   ├── promptRefs ──▶ Prompt(s)
   └── toolRefs ────▶ Tool(s)
```

A `Flow` (or each entry in a `FlowSet`) ties one `Model` together with
one or more `Prompt`s and (optionally) `Tool`s. The `Model` points at a
`PluginConfig` which points at a `Secret`.

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
