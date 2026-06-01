---
title: Contributing
description: How to contribute code, docs, and plugin support.
---

Contributions are very welcome! The project uses Kubebuilder and the
standard `controller-runtime` patterns — if you've worked on a
Kubernetes operator before, you'll feel at home.

## Commit messages

This repo uses [Conventional
Commits](https://www.conventionalcommits.org/). Your messages drive the
next version (see the [release process](/genkit-operator/reference/release/)).

Examples:

- `feat(plugins): add cohere`
- `fix(runner): handle empty prompts directory`
- `docs: clarify FlowSet credentials section`
- `feat!: rename PluginConfig.credentialsRef to authRef`
  (the `!` flags a breaking change)

## Local development with kind

The fastest inner loop is a local [kind](https://kind.sigs.k8s.io/)
cluster — no registry needed, images are loaded straight into the
nodes.

### Prerequisites

- Go 1.25+
- Docker (or Podman — set `CONTAINER_TOOL=podman`)
- [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- `kubectl`

### One-shot bootstrap

```bash
# 1. Create a dedicated cluster (the default name is "genkit").
kind create cluster --name genkit

# 2. Build manager + runner, load both into kind, install CRDs, deploy controller.
make kind-deploy IMG=genkit-operator:dev RUNNER_IMG=genkit-runner:dev
```

`kind-deploy` chains four targets:

| Step | Target | What it does |
| --- | --- | --- |
| 1 | `kind-load` | `docker build` the manager → `kind load docker-image` |
| 2 | `runner-kind-load` | `docker build -f Dockerfile.runner` the runner → load into kind |
| 3 | `install` | `kubectl apply` the CRDs from `config/crd/bases/` |
| 4 | `deploy` | `kustomize build config/default \| kubectl apply -f -` |

Useful overrides:

```bash
make kind-deploy \
  IMG=genkit-operator:dev \
  RUNNER_IMG=genkit-runner:dev \
  KIND_CLUSTER=my-cluster        # default: genkit
```

### Try it out

```bash
# Apply the sample CRs.
kubectl apply -k config/samples/

# Tail the controller logs.
kubectl -n genkit-operator-system logs \
  deploy/genkit-operator-controller-manager -c manager -f

# Watch the Flow rollout the controller creates.
kubectl get flow,deploy,svc -A -w
```

### Iterate on code

After editing controller / runner Go code, rebuild just what changed
and reload it:

```bash
# Manager only
make kind-load IMG=genkit-operator:dev
kubectl -n genkit-operator-system rollout restart \
  deploy/genkit-operator-controller-manager

# Runner only (Flow pods will pick it up on next reconcile;
# force a rollout with `kubectl rollout restart deploy/<flow-name>`)
make runner-kind-load RUNNER_IMG=genkit-runner:dev
```

> `kind load docker-image` only updates the image inside the kind
> nodes — it does **not** restart running pods. Use
> `kubectl rollout restart` (or delete the pod) to pick up a new build.

After editing `*_types.go` or kubebuilder markers:

```bash
make manifests   # regenerate CRDs + RBAC
make generate    # regenerate DeepCopy methods
```

After editing any Go file:

```bash
make lint-fix
make test        # unit tests with envtest (real apiserver + etcd)
```

### Run the controller off-cluster

For the tightest feedback loop, skip the image build entirely and run
the manager on your host against the kind cluster:

```bash
make install                          # CRDs only
make run                              # uses your current kubeconfig context
```

You'll still need the runner image inside kind so that Flow pods can
start:

```bash
make runner-kind-load RUNNER_IMG=genkit-runner:dev
```

### Tear down

```bash
make undeploy           # remove the controller + RBAC
make uninstall          # remove the CRDs
kind delete cluster --name genkit
```

## Adding a plugin

1. Add the import in `cmd/runner/plugins.go`.
2. Register a builder in `pluginRegistry`.
3. (Optional) Add `defaultCredentialKeys` if your provider has a
   conventional env var.
4. Document it under `website/src/content/docs/plugins/<name>.md`.
5. Add a sample under `config/samples/`.
6. Use a `feat(plugins): add <name>` commit.

## Documentation site

The website lives under [`website/`](https://github.com/xavidop/genkit-operator/tree/main/website)
and is an [Astro Starlight](https://starlight.astro.build/) project.

```bash
cd website
npm install
npm run dev      # http://localhost:4321/genkit-operator
```

It is deployed to GitHub Pages on every push to `main` by
[`.github/workflows/docs.yml`](https://github.com/xavidop/genkit-operator/blob/main/.github/workflows/docs.yml).
