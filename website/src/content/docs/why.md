---
title: Why Genkit Operator?
description: The problem the operator solves, and the design choices behind it.
---

[Genkit](https://genkit.dev) is a great framework for building AI features
in Go. But once you want to **run** a Genkit app in production, you end up
writing the same boilerplate over and over:

- A `Dockerfile` per app, even though the only thing that changes is the
  prompts and the model.
- An HTTP server (Gin/Echo/net/http) to expose each flow.
- A `Deployment` + `Service` + `ConfigMap` + `Secret` plumbing for every
  prompt change.
- Provider-specific credential wiring (env vars vs. files, ADC vs. API
  keys, region vs. base URL…).
- Rolling-update logic when a prompt or tool changes.

That's a lot of YAML for "I want to call Claude with this prompt and
serve it at `/greet`."

## What the operator gives you

The Genkit Operator replaces all of that with a small set of **Custom
Resources**:

| Resource       | What it represents                                          |
| -------------- | ----------------------------------------------------------- |
| `PluginConfig` | A provider (anthropic, openai, bedrock, …) + credentials    |
| `Model`        | A specific model + its default generation config            |
| `Prompt`       | A Dotprompt document (YAML frontmatter + Handlebars body)   |
| `Tool`         | A genkit-go `ai.ToolDefinition` + dispatch target           |
| `Flow`         | One model + one or more prompts/tools exposed at one route  |
| `FlowSet`      | Many flows in one Pod, each at `POST /<flow-name>`          |

You apply these. The operator:

1. Resolves references across CRs and reads the referenced `Secret`.
2. Renders the runtime contract into `ConfigMap`s (one per flow).
3. Creates a `Deployment` running the **runner** image (the small
   reference Genkit runtime), mounting prompts/tools/config from the
   `ConfigMap`s and credentials from the `Secret`.
4. Computes a SHA-256 over the rendered content and writes it to the Pod
   template — so any change to a `Prompt`, `Tool`, `Model`, or
   `PluginConfig` triggers a rolling update automatically.
5. Reports status via standard `metav1.Condition`s (`Ready`,
   `Reconciling`, …).

You never write a `Dockerfile`. You never wire up env vars. You never
restart pods manually after editing a prompt.

## Design principles

- **Server-Side Apply everywhere.** Every child resource is applied with
  field manager `genkit-operator`, so you can co-author resources with
  Argo CD / Flux / kubectl without fighting over fields.
- **One Pod, many flows (optional).** `FlowSet` lets multiple flows
  share one runner Pod, with isolated per-flow credentials mounted as
  files — useful for cost-effective fan-out.
- **Pluggable runtime.** The runner image and the on-disk format are a
  documented [contract](/genkit-operator/runtime-contract/). You can
  build your own runner — in Go, Node, Python, whatever — and the
  operator doesn't care.
- **No CRD lock-in.** Each `PluginConfig.extraConfig` is a free-form
  JSON object, so plugin-specific knobs don't require CRD changes.

## When NOT to use it

- You only have **one** Genkit app and you're happy hand-rolling a
  `Deployment`. The operator earns its keep when you have several
  flows, several providers, or a GitOps workflow.
- You need a programming model richer than "prompt → model → HTTP".
  The operator is a deployment tool, not an orchestration framework.
