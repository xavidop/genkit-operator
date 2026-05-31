# Genkit Operator Runtime Contract

This document describes the on-disk and environment contract that the operator
exposes to the user-provided Flow container image.

There are TWO runtime layouts:

1. **Single-flow** — produced by a `Flow` CR. One Pod, one model, one set of
   prompts mounted directly under `/genkit/`.
2. **Multi-flow** — produced by a `FlowSet` CR. One Pod that serves multiple
   logical flows. Per-flow content is mounted under `/genkit/flows/<name>/`
   and a top-level `/genkit/manifest.json` enumerates them.

A runtime image MUST detect the layout by checking whether
`/genkit/manifest.json` exists.

## Single-flow layout (Flow CR)

Every Flow Pod gets a tmpfs-backed projection of three ConfigMaps under
`/genkit`:

```
/genkit/
├── prompts/
│   ├── <prompt-name>.prompt   # raw Dotprompt content (frontmatter + body)
├── tools/
│   ├── <tool-name>.json       # genkit-go Tool descriptor + implementation
└── config.json                # runtime configuration (model + plugin)
```

### `/genkit/prompts/*.prompt`

One file per referenced `Prompt` CR. Filename is the `Prompt`'s
`metadata.name` with the `.prompt` suffix. The file content is the verbatim
value of `spec.content` (a Dotprompt document, i.e. YAML frontmatter + a
Handlebars body).

### `/genkit/tools/*.json`

One file per referenced `Tool` CR. The JSON document follows this shape:

```json
{
  "definition": { /* genkit-go ai.ToolDefinition */ },
  "implementation": {
    "flowRef": { "name": "..." }     // OR
    "http":    { "url": "...", "method": "POST", "headersSecretRef": {"name": "..."} }
  }
}
```

The runtime should treat `implementation` as the dispatch target and use
`definition` to advertise the tool to the model.

### `/genkit/config.json`

A single JSON document with the resolved runtime configuration:

```json
{
  "defaultModel": {
    "provider": "anthropic",
    "model":    "claude-opus-4-6",
    "info":     { /* genkit-go ai.ModelInfo */ },
    "config":   { /* genkit-go ai.GenerationCommonConfig */ }
  },
  "plugin": {
    "type":        "anthropic",
    "region":      "",
    "extraConfig": { /* opaque, passed through from PluginConfig.spec.extraConfig */ }
  }
}
```

Field names follow the standard JSON tags emitted by `genkit-go`.

## Multi-flow layout (FlowSet CR)

A `FlowSet` Pod serves multiple flows from a single container. Filesystem
layout:

```
/genkit/
├── manifest.json                     # enumerates every flow served by this Pod
└── flows/
    ├── <flow-1>/
    │   ├── prompts/<name>.prompt     # one file per Prompt CR
    │   ├── tools/<name>.json         # one file per Tool CR
    │   ├── config.json               # same shape as single-flow + credentialsDir
    │   └── credentials/              # Secret mount; one file per key
    └── <flow-2>/
        └── ...
```

### `/genkit/manifest.json`

```json
{
  "flows": [
    {
      "name":       "greeter-en",
      "entrypoint": "greeting",
      "dir":        "/genkit/flows/greeter-en"
    }
  ]
}
```

* `name` is the flow's HTTP route (`POST /<name>`).
* `entrypoint` is the prompt name (without `.prompt`) invoked by that route.
* `dir` is the absolute path of the flow's content directory.

### `/genkit/flows/<flow>/config.json`

Same fields as the single-flow `config.json` plus:

```json
{
  "credentialsDir": "/genkit/flows/<flow>/credentials",
  "credentialKeys": ["ANTHROPIC_API_KEY"]
}
```

`credentialKeys` is copied from `PluginConfig.spec.credentialKeys`. When
empty, the runtime falls back to the per-plugin defaults listed below.

### `/genkit/flows/<flow>/credentials/`

A read-only mount of the `Secret` named by the flow's
`Model -> PluginConfig.spec.credentialsRef`. One file per key. Runtimes
read credentials by filename (e.g. `ANTHROPIC_API_KEY`) instead of
environment variables, which allows two flows in the same Pod to use
different credentials for the same provider without collision.

### HTTP routing

Each flow is exposed at `POST /<flow-name>`. There is exactly one
HTTP entrypoint per flow — the first prompt in `flows[].prompts`. The
remaining prompts are loaded into the same per-flow registry so the
entrypoint can reference them.

A FlowSet exposes a single `Service` on `FlowSetSpec.port` (default `8080`).

### Content-hash rollout (FlowSet)

The operator SHA-256s every per-flow ConfigMap's data plus the manifest
ConfigMap, in sorted key order, and writes the hex digest to:

* `FlowSet.status.contentHash`
* the Pod template annotation `genkit.dev/content-hash`

## Environment variables

Provider credentials. **Single-flow** Pods receive credentials via `envFrom`
from the `Secret` named by `PluginConfig.spec.credentialsRef`. **Multi-flow**
(FlowSet) Pods receive credentials as files under
`/genkit/flows/<flow>/credentials/`; the runtime reads them from disk
instead of the environment. The runner ships with built-in support for
`anthropic`, `openai`, `googleai`, `vertexai`, `ollama`, and `bedrock`.
Per-plugin defaults (used when `PluginConfig.spec.credentialKeys` is empty):

| Plugin    | Default key(s)                                  | Notes                                              |
| --------- | ----------------------------------------------- | -------------------------------------------------- |
| anthropic | `ANTHROPIC_API_KEY`                             |                                                    |
| openai    | `OPENAI_API_KEY`                                |                                                    |
| googleai  | `GEMINI_API_KEY`, `GOOGLE_API_KEY` (first match)|                                                    |
| vertexai  | `GOOGLE_APPLICATION_CREDENTIALS`                | runner points the env var at the mounted file     |
| ollama    | _(none)_                                        | requires `plugin.extraConfig.serverAddress`        |
| bedrock   | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` | uses AWS SDK default credential chain; `plugin.region` or `plugin.extraConfig.region` selects the region |

Additional environment variables can be appended via `Flow.spec.env`
(single-flow) or `FlowSet.spec.env` (multi-flow).

## Network

The container is expected to listen on `Flow.spec.port` (default `8080`).
A `Service` of `Flow.spec.serviceType` (default `ClusterIP`) is created with
the same port. The DNS name is `<flow>.<namespace>.svc`.

## Content-hash rollout

The operator computes a SHA-256 over the sorted contents of the three
ConfigMaps and writes it to:

* `Flow.status.contentHash`
* the Pod template annotation `genkit.dev/content-hash`

Any change to a referenced Prompt, Tool, Model, or PluginConfig (Secret name
included) causes the hash to change, which in turn triggers a rolling update
of the Deployment.

## Ownership and SSA

All child resources are created via Server-Side Apply with field manager
`genkit-operator` and an `OwnerReference` pointing at the parent CR. Garbage
collection is delegated to Kubernetes.
