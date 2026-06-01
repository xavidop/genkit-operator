---
title: Vertex AI
description: Use Gemini and partner models on Google Cloud Vertex AI.
---

Vertex AI uses Google Application Default Credentials (ADC). The runner
points `GOOGLE_APPLICATION_CREDENTIALS` at the mounted service-account
key file.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vertex-credentials
type: Opaque
data:
  # key name MUST be GOOGLE_APPLICATION_CREDENTIALS; value is a
  # base64-encoded JSON service account key.
  GOOGLE_APPLICATION_CREDENTIALS: <base64>
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: vertexai
spec:
  type: vertexai
  region: us-central1
  credentialsRef:
    name: vertex-credentials
  extraConfig:
    projectId: my-gcp-project
    # location takes precedence over the top-level region field
    # location: us-central1
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: gemini-2-pro
spec:
  provider: vertexai
  model: gemini-2.0-pro
  pluginConfigRef:
    name: vertexai
```

## Credentials

| Default key                       | Notes |
| --------------------------------- | ----- |
| `GOOGLE_APPLICATION_CREDENTIALS`  | Path to the SA key; the runner sets the env var for the SDK |

## `extraConfig`

| Field       | Type   | Description                                  |
| ----------- | ------ | -------------------------------------------- |
| `projectId` | string | GCP project ID                               |
| `location`  | string | GCP region; overrides top-level `region`     |

## Reference

- `genkit-go` plugin: [`go/plugins/googlegenai`](https://github.com/firebase/genkit/tree/main/go/plugins/googlegenai)
- Models on Vertex: [`cloud.google.com/vertex-ai/generative-ai/docs/learn/models`](https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models)
