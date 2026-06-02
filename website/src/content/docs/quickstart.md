---
title: Quickstart
description: Install the operator and deploy your first Flow in under five minutes.
---

This walks through installing the Genkit Operator and deploying a sample
Flow backed by Anthropic Claude.

## Prerequisites

- A Kubernetes cluster (v1.27+). [`kind`](https://kind.sigs.k8s.io/) is
  fine for local testing.
- `kubectl` and (optionally) `helm` 3.16+.
- An Anthropic API key (or credentials for any other supported
  provider).

## 1. Install the operator

The fastest path is the published Helm chart:

```bash
helm install genkit-operator \
  oci://ghcr.io/xavidop/charts/genkit-operator \
  --version {{LATEST_VERSION}} \
  --namespace genkit-operator-system --create-namespace
```

Prefer plain YAML? Use the bundled installer attached to each GitHub
Release:

```bash
kubectl apply -f https://github.com/xavidop/genkit-operator/releases/latest/download/install.yaml
```

Verify the controller is up:

```bash
kubectl -n genkit-operator-system get pods
kubectl get crds | grep genkit.dev
```

## 2. Create provider credentials

```bash
kubectl create secret generic anthropic-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

## 3. Apply the sample CRs

The repository ships a complete end-to-end example under
`config/samples/`:

```bash
kubectl apply -f https://raw.githubusercontent.com/xavidop/genkit-operator/main/config/samples/genkit_v1alpha1_pluginconfig.yaml
kubectl apply -f https://raw.githubusercontent.com/xavidop/genkit-operator/main/config/samples/genkit_v1alpha1_model.yaml
kubectl apply -f https://raw.githubusercontent.com/xavidop/genkit-operator/main/config/samples/genkit_v1alpha1_prompt.yaml
kubectl apply -f https://raw.githubusercontent.com/xavidop/genkit-operator/main/config/samples/genkit_v1alpha1_tool.yaml
kubectl apply -f https://raw.githubusercontent.com/xavidop/main/config/samples/genkit_v1alpha1_flow.yaml
```

## 4. Watch reconciliation

```bash
kubectl get gpc,gmd,gpr,gtl,gfl
kubectl describe gfl greeter
kubectl get deploy,svc,cm -l app.kubernetes.io/managed-by=genkit-operator
```

Once the `Deployment` is ready, the `Flow` reports `Phase=Running` and a
`Ready=True` condition.

## 5. Call your flow

```bash
kubectl port-forward svc/greeter 8080:8080 &
curl -s -X POST http://localhost:8080/greeter \
  -H 'content-type: application/json' \
  -d '{"name":"world"}'
```

## 6. Roll out a content change

Edit the `Prompt` body and reapply:

```bash
kubectl edit prompt greeting
```

The operator recomputes the content hash, updates the Pod template
annotation `genkit.dev/content-hash`, and triggers a rolling update of
the `Flow` Deployment automatically.

## Next steps

- Read the [runtime contract](/genkit-operator/runtime-contract/) to
  learn how a runner consumes `/genkit/*`.
- Use [`FlowSet`](/genkit-operator/guides/flowset/) to deploy several
  related flows in one Pod.
- Add another provider — try [AWS Bedrock](/genkit-operator/plugins/bedrock/).
