---
title: Architecture
description: How the operator turns YAML into a running, addressable Genkit HTTP service.
---

## High-level

```
Dev (writes YAML)        ┐
                         │
                         ▼
                  ┌────────────────┐       ┌──────────────────────┐
                  │ Kubernetes API │──────▶│ genkit-operator      │
                  └────────────────┘       │ (controller-manager) │
                                           └──────────┬───────────┘
                                                      │ SSA
                                                      ▼
                                  ┌────────────────────────────────┐
                                  │ Deployment + Service +         │
                                  │ ConfigMaps (manifest, prompts, │
                                  │ tools, config) + Secret mount  │
                                  └──────────┬─────────────────────┘
                                             │
                                             ▼
                                  ┌────────────────────────────────┐
                                  │ Runner Pod                     │
                                  │ reads /genkit/* → POST /<flow> │
                                  └────────────────────────────────┘
```

You hand the API server a set of CRs:

- `PluginConfig` (which provider + credentials `Secret`)
- `Model` (which model + default config + `pluginConfigRef`)
- `Prompt`s and `Tool`s
- `Flow` (single endpoint) **or** `FlowSet` (many endpoints in one Pod)

The controller resolves references, renders the
[runtime contract](/runtime-contract/) into
`ConfigMap`s, applies a `Deployment` and `Service`, and writes status
back.

## Reconciliation flow

For each `Flow` / `FlowSet`:

1. **Resolve refs.** Look up the `Model`, every `Prompt` and `Tool`,
   and the `PluginConfig` (plus its `Secret`). Any missing ref blocks
   the reconciliation with a clear `Conditions` entry.
2. **Render.** Build the on-disk layout the runner expects:
   - `config.json` — resolved model + plugin config
   - `prompts/<name>.prompt` — verbatim Dotprompt
   - `tools/<name>.json` — `ai.ToolDefinition` + dispatch target
   - `manifest.json` — only for `FlowSet`
3. **SHA-256 the content.** This hash is written to:
   - `Flow.status.contentHash` / `FlowSet.status.contentHash`
   - The Pod template annotation `genkit.dev/content-hash`
4. **Server-Side Apply** all child resources with field manager
   `genkit-operator` and owner references back to the parent CR.
5. **Update status.** `Phase`, `Conditions` (`Ready`, `Reconciling`),
   `ObservedGeneration`.

## Why content hashing matters

Kubernetes will not restart Pods when a mounted `ConfigMap`'s data
changes. The annotation flip *is* the trigger:

1. You edit a `Prompt`.
2. The `Prompt` controller reconciles; nothing in the `Flow`'s
   `Deployment` changes yet.
3. The `Flow` controller (which watches `Prompt`) re-renders.
4. The new content hash differs → annotation changes → Deployment
   sees a new Pod template → rolling update.

No other mechanism in the operator forces a restart.

## Single-Flow vs FlowSet

| Aspect              | `Flow`                                | `FlowSet`                            |
| ------------------- | ------------------------------------- | ------------------------------------ |
| HTTP routes / Pod   | 1 (`POST /<flow-name>`)               | N (one per flow)                     |
| ConfigMaps          | 3 (`prompts`, `tools`, `config`)      | 1 manifest + 3 per flow              |
| Credentials         | `envFrom` on the Pod                  | One file mount per flow under `…/credentials/` |
| Use case            | One model, one purpose                | Multiple flows sharing a Pod, possibly different providers |

## Ownership and GC

Every child resource has an `OwnerReference` pointing at the parent
CR. Deleting the `Flow` / `FlowSet` triggers Kubernetes garbage
collection of all rendered resources.

## What the operator does NOT do

- It does not embed an LLM proxy. Calls go directly from the runner
  Pod to the provider.
- It does not manage `Secret` contents. You bring them; the operator
  references them.
- It does not enforce network policies, mTLS, or auth on the flow's
  HTTP endpoint. Wrap the `Service` in your usual ingress / service
  mesh.
