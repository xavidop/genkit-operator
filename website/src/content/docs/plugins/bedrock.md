---
title: AWS Bedrock
description: Use Anthropic Claude, Amazon Nova, Meta Llama, Mistral and others through AWS Bedrock.
---

The Bedrock plugin is provided by
[`github.com/xavidop/genkit-aws-bedrock-go`](https://github.com/genkit-ai/aws-bedrock-go-plugin).
The reference runner registers it as the `bedrock` plugin type.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bedrock-credentials
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: ...
  AWS_SECRET_ACCESS_KEY: ...
  # Optional, only for temporary credentials (e.g. STS / AssumeRole)
  # AWS_SESSION_TOKEN: ...
---
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: bedrock
spec:
  type: bedrock
  region: us-east-1
  credentialsRef:
    name: bedrock-credentials
  credentialKeys:
    - AWS_ACCESS_KEY_ID
    - AWS_SECRET_ACCESS_KEY
    # - AWS_SESSION_TOKEN
---
apiVersion: genkit.dev/v1alpha1
kind: Model
metadata:
  name: bedrock-claude-haiku
spec:
  provider: bedrock
  model: anthropic.claude-3-haiku-20240307-v1:0
  pluginConfigRef:
    name: bedrock
  defaultConfig:
    temperature: 0.3
    maxOutputTokens: 1024
```

## Credentials

| Default key             | Notes                                            |
| ----------------------- | ------------------------------------------------ |
| `AWS_ACCESS_KEY_ID`     | Standard AWS access key                          |
| `AWS_SECRET_ACCESS_KEY` | Standard AWS secret key                          |
| `AWS_SESSION_TOKEN`     | Optional, for temporary credentials (STS, SSO)   |

In FlowSet mode the runner reads the mounted credential files and
exports them to the process environment so the AWS SDK's default
credential chain picks them up. You can also run on EKS with IRSA / Pod
Identity and leave `credentialsRef` pointing at an empty Secret — in
that case do not set `credentialKeys`.

## Region

Set `spec.region` (preferred) or `spec.extraConfig.region`. The
`extraConfig` value takes precedence.

## Reference

- Plugin: [`github.com/xavidop/genkit-aws-bedrock-go`](https://github.com/genkit-ai/aws-bedrock-go-plugin)
- Model access: AWS console → Bedrock → Model access (must be granted
  per model, per region).
- IAM: at minimum `bedrock:InvokeModel` and
  `bedrock:InvokeModelWithResponseStream` on the foundation models you
  use.
