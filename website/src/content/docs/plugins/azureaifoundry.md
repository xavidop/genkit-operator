---
title: Azure AI Foundry
description: Use GPT-5, GPT-4o, embeddings, DALL-E, TTS and Whisper through Azure AI Foundry / Azure OpenAI.
---

The Azure AI Foundry plugin is provided by
[`github.com/xavidop/genkit-azure-foundry-go`](https://github.com/xavidop/genkit-azure-foundry-go).
The reference runner registers it as the `azureaifoundry` plugin type.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azureaifoundry-credentials
type: Opaque
stringData:
  AZURE_OPENAI_API_KEY: ...
  # Optional: provide the endpoint via the Secret instead of extraConfig
  # AZURE_OPENAI_ENDPOINT: https://your-resource.openai.azure.com/
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: azureaifoundry
spec:
  type: azureaifoundry
  credentialsRef:
    name: azureaifoundry-credentials
  credentialKeys:
    - AZURE_OPENAI_API_KEY
  extraConfig:
    endpoint: https://your-resource.openai.azure.com/
    # apiVersion: "2024-02-15-preview"   # optional
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: azureaifoundry-gpt-4o
spec:
  provider: azureaifoundry
  # Use your Azure *deployment* name (not the underlying model name).
  model: gpt-4o
  pluginConfigRef:
    name: azureaifoundry
  defaultConfig:
    temperature: 0.3
    maxOutputTokens: 1024
```

## Credentials

| Default key            | Notes                                                      |
| ---------------------- | ---------------------------------------------------------- |
| `AZURE_OPENAI_API_KEY` | API key for your Azure OpenAI / AI Foundry resource        |

`AZURE_OPENAI_ENDPOINT` is also accepted as a credential key when you'd
rather keep the endpoint in the Secret than in `extraConfig`. In FlowSet
mode the runner reads the mounted credential files; in single-flow mode
they're injected via `envFrom`.

## Endpoint & API version

| Field                         | Required | Notes                                          |
| ----------------------------- | -------- | ---------------------------------------------- |
| `extraConfig.endpoint`        | yes\*    | Azure OpenAI endpoint URL (include trailing `/`) |
| `extraConfig.apiVersion`      | no       | Azure OpenAI REST API version; defaults to the plugin's latest |

\* The endpoint can alternatively come from the `AZURE_OPENAI_ENDPOINT`
credential key. If neither is set, the runner refuses to start.

## Model deployments

In Azure, the `model` field is your **deployment name**, not the
underlying model name. For example, if you deployed `gpt-4o` with the
deployment name `my-gpt4o-deployment`, set `spec.model: my-gpt4o-deployment`.

## Reference

- Plugin: [`github.com/xavidop/genkit-azure-foundry-go`](https://github.com/xavidop/genkit-azure-foundry-go)
- Authentication via `azcore.TokenCredential` (Managed Identity,
  DefaultAzureCredential, Service Principal, Azure CLI) is supported by
  the upstream plugin but not yet wired through the reference runner —
  use the API-key path or [build a custom runner](/genkit-operator/guides/custom-runner/).
