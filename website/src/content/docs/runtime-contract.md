---
title: Runtime contract
description: The on-disk and environment contract every runner image must satisfy.
---

This page describes the on-disk and environment contract that the
operator exposes to the user-provided Flow container image (the
**runner**). Anything that follows this contract is a valid runner —
the reference Go runtime in `cmd/runner` is one implementation, but
nothing stops you from writing your own in Node, Python, Rust, etc.

There are TWO runtime layouts:

1. **Single-flow** — produced by a `Flow` CR. One Pod, one model, one
   set of prompts mounted directly under `/genkit/`.
2. **Multi-flow** — produced by a `FlowSet` CR. One Pod that serves
   multiple logical flows. Per-flow content is mounted under
   `/genkit/flows/<name>/` and a top-level `/genkit/manifest.json`
   enumerates them.

A runtime image MUST detect the layout by checking whether
`/genkit/manifest.json` exists.

## Single-flow layout (`Flow` CR)

Every Flow Pod gets a tmpfs-backed projection of three ConfigMaps
under `/genkit`:

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
`metadata.name` with the `.prompt` suffix. The file content is the
verbatim value of `spec.content` (a Dotprompt document, i.e. YAML
frontmatter + a Handlebars body).

### `/genkit/tools/*.json`

One file per referenced `Tool` CR. The JSON document follows this shape:

```json
{
  "definition": { /* genkit-go ai.ToolDefinition */ },
  "implementation": {
    "flowRef": { "name": "..." },
    "http":    { "url": "...", "method": "POST", "headersSecretRef": {"name": "..."} }
  }
}
```

The runtime should treat `implementation` as the dispatch target and
use `definition` to advertise the tool to the model.

### `/genkit/config.json`

```json
{
  "defaultModel": {
    "provider": "anthropic",
    "model":    "claude-opus-4-6",
    "info":     { /* ai.ModelInfo */ },
    "config":   { /* ai.GenerationCommonConfig */ }
  },
  "plugin": {
    "type":        "anthropic",
    "region":      "",
    "extraConfig": { /* opaque PluginConfig.spec.extraConfig */ }
  }
}
```

Field names follow the standard JSON tags emitted by `genkit-go`.

## Multi-flow layout (`FlowSet` CR)

A `FlowSet` Pod serves multiple flows from a single container:

```
/genkit/
├── manifest.json
└── flows/
    ├── <flow-1>/
    │   ├── prompts/<name>.prompt
    │   ├── tools/<name>.json
    │   ├── config.json
    │   └── credentials/      # Secret mount; one file per key
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

- `name` is the flow's HTTP route (`POST /<name>`).
- `entrypoint` is the prompt name (without `.prompt`) invoked by that
  route.
- `dir` is the absolute path of the flow's content directory.

### `/genkit/flows/<flow>/config.json`

Same fields as the single-flow `config.json` plus:

```json
{
  "credentialsDir": "/genkit/flows/<flow>/credentials",
  "credentialKeys": ["ANTHROPIC_API_KEY"]
}
```

### `/genkit/flows/<flow>/credentials/`

A read-only mount of the `Secret` named by the flow's
`Model → PluginConfig.spec.credentialsRef`. One file per key. Runtimes
read credentials by filename instead of environment variables, which
allows two flows in the same Pod to use different credentials for the
same provider without collision.

## Credentials by plugin

| Plugin    | Default key(s)                                                   | Notes                                            |
| --------- | ---------------------------------------------------------------- | ------------------------------------------------ |
| anthropic | `ANTHROPIC_API_KEY`                                              |                                                  |
| openai    | `OPENAI_API_KEY`                                                 |                                                  |
| googleai  | `GEMINI_API_KEY`, `GOOGLE_API_KEY` (first match)                 |                                                  |
| vertexai  | `GOOGLE_APPLICATION_CREDENTIALS`                                 | runner points the env var at the mounted file    |
| ollama    | _(none)_                                                         | requires `plugin.extraConfig.serverAddress`      |
| bedrock   | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`| AWS SDK default credential chain; region from `plugin.region` or `extraConfig.region` |
| azureaifoundry | `AZURE_OPENAI_API_KEY`                                      | requires `plugin.extraConfig.endpoint` (or `AZURE_OPENAI_ENDPOINT` key); optional `extraConfig.apiVersion` |

**Single-flow** Pods receive credentials via `envFrom`. **FlowSet**
Pods receive credentials as files under
`/genkit/flows/<flow>/credentials/`.

## Network

The container is expected to listen on `Flow.spec.port` (default `8080`).
A `Service` of `Flow.spec.serviceType` (default `ClusterIP`) is created
with the same port. The DNS name is `<flow>.<namespace>.svc`.

## Content-hash rollout

The operator SHA-256s every rendered ConfigMap's data (in sorted key
order) and writes the hex digest to:

- `Flow.status.contentHash` / `FlowSet.status.contentHash`
- the Pod template annotation `genkit.dev/content-hash`

Any change to a referenced `Prompt`, `Tool`, `Model`, or `PluginConfig`
(including the referenced `Secret` name) causes the hash to change,
which in turn triggers a rolling update of the Deployment.

## Ownership and SSA

All child resources are created via Server-Side Apply with field
manager `genkit-operator` and an `OwnerReference` pointing at the
parent CR. Garbage collection is delegated to Kubernetes.
