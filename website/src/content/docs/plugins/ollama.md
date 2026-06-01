---
title: Ollama
description: Run local models via an Ollama server.
---

The Ollama plugin requires no credentials — only a reachable Ollama
server address.

```yaml
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: ollama
spec:
  type: ollama
  credentialsRef:
    name: ollama-empty   # any Secret name; can be empty
  extraConfig:
    serverAddress: "http://ollama.ollama.svc.cluster.local:11434"
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: llama-3
spec:
  provider: ollama
  model: llama3:8b
  pluginConfigRef:
    name: ollama
```

## `extraConfig`

| Field           | Type   | Description                              |
| --------------- | ------ | ---------------------------------------- |
| `serverAddress` | string | **Required.** HTTP URL of an Ollama server. |

## Notes

- The flow Pod must have network reach to `serverAddress`. Run Ollama
  in the same cluster (e.g. as a `Deployment` + `Service`) for the
  simplest setup.
- `credentialsRef` is required by the CRD schema but unused — point it
  at any empty `Secret`.

## Reference

- `genkit-go` plugin: [`go/plugins/ollama`](https://github.com/firebase/genkit/tree/main/go/plugins/ollama)
- Ollama: [`ollama.com`](https://ollama.com/)
