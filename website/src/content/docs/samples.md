---
title: Sample CRs
description: A guided tour of every YAML under config/samples/.
---

The repository ships a complete sample suite under
[`config/samples/`](https://github.com/xavidop/genkit-operator/tree/main/config/samples).
Apply them in any order — the controller will retry until cross-CR
references resolve.

## Apply everything

```bash
kubectl apply -k https://github.com/xavidop/genkit-operator//config/samples?ref=main
```

## Per-resource highlights

### `genkit_v1alpha1_pluginconfig.yaml`

The default sample uses Anthropic. The companion
[`genkit_v1alpha1_pluginconfig_bedrock.yaml`](https://github.com/xavidop/genkit-operator/blob/main/config/samples/genkit_v1alpha1_pluginconfig_bedrock.yaml)
shows the same pattern for AWS Bedrock, and
[`genkit_v1alpha1_pluginconfig_azureaifoundry.yaml`](https://github.com/xavidop/genkit-operator/blob/main/config/samples/genkit_v1alpha1_pluginconfig_azureaifoundry.yaml)
for Azure AI Foundry.

### `genkit_v1alpha1_model.yaml`

References the `anthropic` plugin and sets default generation config
(temperature, max tokens, top-p). Override these per-prompt via the
Dotprompt frontmatter.

### `genkit_v1alpha1_prompt.yaml`

A Dotprompt document — YAML frontmatter (`model: …`, `temperature: …`)
followed by a Handlebars body. The runner re-renders this on every
request with the JSON payload as the template context.

### `genkit_v1alpha1_tool.yaml`

A `genkit-go` `ai.ToolDefinition` plus an HTTP dispatch target. The
runner surfaces the tool to the model and proxies tool calls to your
service.

### `genkit_v1alpha1_flow.yaml`

The single-Flow sample. Wires everything together and exposes
`POST /greeter`.

### `genkit_v1alpha1_flowset.yaml`

The multi-flow sample. Three flows in one Pod, each at its own route.

### `genkit_v1alpha1_dataset.yaml` and `genkit_v1alpha1_eval.yaml`

A scheduled evaluation pipeline:

- `Dataset` enumerates `{input, reference}` examples.
- `Eval` runs that dataset against a `Flow` on a `cron` schedule and
  emits a Kubernetes `Job`.

## Adapting the samples

Each sample uses `metadata.namespace: default`. Drop or change that
field if you keep your workloads in a dedicated namespace, and remember
that **all** cross-CR references are resolved within the same
namespace — there is no cross-namespace reference.
