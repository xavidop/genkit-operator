---
title: Anthropic
description: Use Claude models via the Anthropic API.
---

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: anthropic-credentials
type: Opaque
stringData:
  ANTHROPIC_API_KEY: sk-ant-...
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: anthropic
spec:
  type: anthropic
  credentialsRef:
    name: anthropic-credentials
  credentialKeys: [ANTHROPIC_API_KEY]
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: claude-opus
spec:
  provider: anthropic
  model: claude-opus-4-6
  pluginConfigRef:
    name: anthropic
  defaultConfig:
    temperature: 0.3
    maxOutputTokens: 1024
```

## Credentials

| Default key         | Notes |
| ------------------- | ----- |
| `ANTHROPIC_API_KEY` | Standard Anthropic API key |

## Reference

- `genkit-go` plugin: [`go/plugins/anthropic`](https://github.com/firebase/genkit/tree/main/go/plugins/anthropic)
- Model catalog: [`docs.anthropic.com`](https://docs.anthropic.com/en/docs/models-overview)
