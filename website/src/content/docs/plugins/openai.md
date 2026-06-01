---
title: OpenAI
description: Use GPT models via the OpenAI API (or any compat-OAI endpoint).
---

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openai-credentials
type: Opaque
stringData:
  OPENAI_API_KEY: sk-...
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: openai
spec:
  type: openai
  credentialsRef:
    name: openai-credentials
  credentialKeys: [OPENAI_API_KEY]
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: gpt-4o
spec:
  provider: openai
  model: gpt-4o
  pluginConfigRef:
    name: openai
```

## Credentials

| Default key      | Notes |
| ---------------- | ----- |
| `OPENAI_API_KEY` | Standard OpenAI API key |

## Reference

- `genkit-go` plugin: [`go/plugins/compat_oai/openai`](https://github.com/firebase/genkit/tree/main/go/plugins/compat_oai/openai)
- Model catalog: [`platform.openai.com/docs/models`](https://platform.openai.com/docs/models)
