---
title: Plugins overview
description: How PluginConfig maps to a provider, and which providers ship with the reference runner.
---

A `PluginConfig` describes a Genkit model provider: a type, a region
(optional), a `Secret` reference with credentials, and optional
provider-specific knobs in `extraConfig`.

```yaml
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: my-provider
spec:
  type: <provider-name>           # required
  region: <region>                # optional
  credentialsRef:
    name: <secret-name>           # required
  credentialKeys:                 # optional, defaults per provider
    - KEY_1
  extraConfig:                    # opaque, passed to the runner as JSON
    foo: bar
```

The reference runner (`cmd/runner`) ships with built-in builders for the
following providers:

| Type        | Backed by                                       | Page                                                  |
| ----------- | ----------------------------------------------- | ----------------------------------------------------- |
| `anthropic` | `genkit-go/plugins/anthropic`                   | [Anthropic](/genkit-operator/plugins/anthropic/)      |
| `openai`    | `genkit-go/plugins/compat_oai/openai`           | [OpenAI](/genkit-operator/plugins/openai/)            |
| `googleai`  | `genkit-go/plugins/googlegenai` (`GoogleAI`)    | [Google AI](/genkit-operator/plugins/googleai/)       |
| `vertexai`  | `genkit-go/plugins/googlegenai` (`VertexAI`)    | [Vertex AI](/genkit-operator/plugins/vertexai/)       |
| `ollama`    | `genkit-go/plugins/ollama`                      | [Ollama](/genkit-operator/plugins/ollama/)            |
| `bedrock`   | `github.com/xavidop/genkit-aws-bedrock-go`      | [AWS Bedrock](/genkit-operator/plugins/bedrock/)      |

## Adding a new provider

The plugin registry is one map in
[`cmd/runner/plugins.go`](https://github.com/xavidop/blob/main/cmd/runner/plugins.go).
Adding a provider is a one-liner plus a builder function:

```go
var pluginRegistry = map[string]pluginBuilder{
    "anthropic": buildAnthropic,
    "openai":    buildOpenAI,
    // ...
    "myprov":    buildMyProv,
}

func buildMyProv(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
    key, err := requireCredential("myprov", credentialsDir, keys)
    if err != nil { return nil, err }
    return &myprov.MyProv{APIKey: key}, nil
}
```

If you'd rather not rebuild the runner, [build your own
runner](/genkit-operator/guides/custom-runner/) image with whatever
provider stack you want.

## Free-form `extraConfig`

`PluginConfig.spec.extraConfig` is `apiextensionsv1.JSON` — the CRD
preserves unknown fields. The controller passes it through verbatim to
the runner's `config.json` under `plugin.extraConfig`. This lets each
provider read its own opaque settings (base URL, project ID, region,
server address, …) without requiring CRD schema changes.
