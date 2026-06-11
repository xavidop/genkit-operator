---
title: Deploy a Flow
description: From zero to a callable HTTP endpoint, step by step.
---

This guide deploys a single Flow exposed at `POST /greeter`, backed by
Anthropic Claude.

## 1. Credentials

```bash
kubectl create secret generic anthropic-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

## 2. Plugin

```yaml
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: anthropic
spec:
  type: anthropic
  credentialsRef:
    name: anthropic-credentials
  credentialKeys: [ANTHROPIC_API_KEY]
```

## 3. Model

```yaml
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

## 4. Prompt

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Prompt
metadata:
  name: greeting
spec:
  content: |
    ---
    model: anthropic/claude-opus-4-6
    temperature: 0.3
    ---
    Greet the user named {{name}} in a single sentence.
```

## 5. Flow

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelRef: { name: claude-opus }
  promptRefs:
    - { name: greeting }
  port: 8080
```

## 6. Apply and verify

```bash
kubectl apply -f .
kubectl get gfl greeter
kubectl describe gfl greeter   # check Conditions
```

When `Ready=True`:

```bash
kubectl port-forward svc/greeter 8080:8080 &
curl -s -X POST http://localhost:8080/greeter \
  -H 'content-type: application/json' \
  -d '{"name":"Ada"}'
```

## 7. Iterate

Edit the `Prompt`, reapply, and watch the rolling update:

```bash
kubectl edit prompt greeting
kubectl rollout status deploy/greeter
```

You don't need to bump anything else — the controller recomputes the
content hash and patches the Deployment's Pod template annotation
automatically.

## Inline model spec and prompts (no Model or Prompt CRs)

If you prefer to keep everything in one manifest — or want to avoid
creating separate `Model` and `Prompt` CRs — you can embed both the
model definition and the prompt content directly in the `Flow`.

You still need a `PluginConfig` (and its `Secret`) for credentials;
only the `Model` and `Prompt` CRs become optional.

### 1. Credentials and plugin (same as before)

```bash
kubectl create secret generic anthropic-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

```yaml
apiVersion: genkit.dev/v1alpha1
kind: PluginConfig
metadata:
  name: anthropic-config
spec:
  type: anthropic
  credentialsRef:
    name: anthropic-credentials
  credentialKeys: [ANTHROPIC_API_KEY]
```

### 2. Flow with inline model spec and inline prompt

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/xavidop/genkit-runner:{{LATEST_TAG}}
  modelSpec:
    provider: anthropic
    model: claude-opus-4-7
    pluginConfigRef:
      name: anthropic-config
    defaultConfig:
      temperature: 0.3
      maxOutputTokens: 1024
  prompts:
    - prompt:
        name: greeting
        content: |
          ---
          model: anthropic/claude-opus-4-7
          ---
          Greet the user named {{name}} in a single sentence.
  port: 8080
```

`modelSpec` and `modelRef` are mutually exclusive — pick one. Likewise,
each item in `prompts` uses either `promptRef` (a reference to a
`Prompt` CR) or `prompt` (inline content), not both.

### 3. Apply and call

```bash
kubectl apply -f plugin-config.yaml -f flow-inline.yaml
kubectl get gfl greeter

kubectl port-forward svc/greeter 8080:8080 &
curl -s -X POST http://localhost:8080/greeter \
  -H 'content-type: application/json' \
  -d '{"name":"Ada"}'
```
