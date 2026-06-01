---
title: Google AI (Gemini)
description: Use Gemini via the Google AI / Generative Language API.
---

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: googleai-credentials
type: Opaque
stringData:
  GEMINI_API_KEY: ...
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: googleai
spec:
  type: googleai
  credentialsRef:
    name: googleai-credentials
  # credentialKeys defaults to GEMINI_API_KEY then GOOGLE_API_KEY
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: gemini-2-flash
spec:
  provider: googleai
  model: gemini-2.0-flash
  pluginConfigRef:
    name: googleai
```

## Credentials

| Default key        | Notes                                          |
| ------------------ | ---------------------------------------------- |
| `GEMINI_API_KEY`   | Preferred                                      |
| `GOOGLE_API_KEY`   | Fallback if `GEMINI_API_KEY` is not present    |

## Reference

- `genkit-go` plugin: [`go/plugins/googlegenai`](https://github.com/firebase/genkit/tree/main/go/plugins/googlegenai)
- Model catalog: [`ai.google.dev/gemini-api/docs/models`](https://ai.google.dev/gemini-api/docs/models)
