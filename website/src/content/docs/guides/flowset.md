---
title: FlowSet — multiple flows in one Pod
description: Use FlowSet to consolidate related flows behind one Service, with per-flow credentials.
---

A `FlowSet` serves several flows from a single runner Pod. Each flow
gets its own per-flow ConfigMaps under `/genkit/flows/<name>/` and its
own credentials mount, so two flows can use the same provider with
*different* credentials without collisions.

## When to reach for FlowSet

- You have many small flows, and one Pod per flow would be wasteful.
- Several flows share most of their dependencies and you want a single
  rolling-update unit.
- You want per-flow credentials isolated as files instead of env vars.

If a flow is high-traffic or needs its own scaling profile, keep it as
a standalone `Flow`.

## Example

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
    - name: translator
      modelRef: { name: claude-haiku }
      promptRefs:
        - { name: translate }
```

Routes exposed by the single Pod:

- `POST /greeter`
- `POST /summarizer`
- `POST /translator`

## How credentials are mounted

Each flow's `PluginConfig.credentialsRef` Secret is mounted at:

```
/genkit/flows/<flow-name>/credentials/
```

One file per key. So with the manifest above, the Pod will have:

```
/genkit/
├── manifest.json
└── flows/
    ├── greeter/
    │   ├── prompts/greeting.prompt
    │   ├── config.json
    │   └── credentials/
    │       └── ANTHROPIC_API_KEY
    ├── summarizer/
    │   └── ...
    └── translator/
        └── ...
```

A custom runner only needs to read the right credentials per request
(by looking at the route). The reference runner does this automatically.

## Content-hash rollout

Just like `Flow`, the operator computes a SHA-256 over all per-flow
ConfigMaps plus the manifest and writes it to the Pod template
annotation `genkit.dev/content-hash`. Editing any referenced `Prompt`,
`Tool`, `Model`, or `PluginConfig` triggers a rolling update of the
single shared Deployment.
