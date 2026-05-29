# Quickstart

This walks through installing the Genkit Operator and deploying a sample
Flow backed by Anthropic Claude Opus 4.7.

## 1. Install the operator

Using the bundled Helm chart:

```bash
make manifests generate
helm install genkit-operator dist/chart \
  --namespace genkit-system --create-namespace
```

Or with kustomize / `make`:

```bash
make install        # CRDs
make deploy IMG=ghcr.io/xavidop/genkit-operator:latest
```

Verify:

```bash
kubectl -n genkit-system get pods
kubectl get crds | grep genkit.dev
```

## 2. Create provider credentials

```bash
kubectl create secret generic anthropic-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

## 3. Apply the samples

The `config/samples/` directory contains a complete end-to-end example using
Claude Opus 4.7:

```bash
kubectl apply -f config/samples/genkit_v1alpha1_pluginconfig.yaml
kubectl apply -f config/samples/genkit_v1alpha1_model.yaml
kubectl apply -f config/samples/genkit_v1alpha1_prompt.yaml
kubectl apply -f config/samples/genkit_v1alpha1_tool.yaml
kubectl apply -f config/samples/genkit_v1alpha1_flow.yaml
```

## 4. Watch reconciliation

```bash
kubectl get gpc,gmd,gpr,gtl,gfl
kubectl describe gfl greeter
kubectl get deploy,svc,cm -l app.kubernetes.io/managed-by=genkit-operator
```

Once the Deployment is ready, the Flow status reports `Phase=Running` and a
`Ready=True` condition.

## 5. Run an evaluation

```bash
kubectl apply -f config/samples/genkit_v1alpha1_dataset.yaml
kubectl apply -f config/samples/genkit_v1alpha1_eval.yaml
kubectl get gev,cronjob,jobs
```

## 6. Roll out a content change

Edit the Prompt body and reapply:

```bash
kubectl edit prompt greeting
```

The operator recomputes the content hash, updates the Pod template
annotation `genkit.dev/content-hash`, and triggers a rolling update of the
Flow Deployment automatically.

## Next steps

* Read the [runtime contract](runtime-contract.md) to learn how to build
  a Flow container image that consumes `/genkit/*`.
* Use `FlowSet` to deploy fleets of related Flows that share defaults.
