# genkit-operator

A Kubernetes operator that turns a handful of YAML files into
production-ready [Genkit](https://genkit.dev) (Go) HTTP endpoints —
no Dockerfiles to write, no servers to wire up, no credential plumbing
to maintain. Declare your prompts, models, tools and flows as Custom
Resources and the operator assembles, deploys and serves them for you.

## Description

`genkit-operator` is the easiest way to run [Genkit](https://genkit.dev)
flows on Kubernetes. Instead of building your own image and writing the
boilerplate to load prompts, register plugins and serve HTTP, you submit
a small set of Custom Resources — `PluginConfig`, `Model`, `Prompt`,
`Tool`, `Flow` / `FlowSet`, and optionally `Dataset` / `Eval` — and the
operator does the rest:

- Resolves all references and renders a normalized `manifest.json` plus
  one `config.json` per flow into ConfigMaps.
- Mounts provider credentials from your own `Secrets` into the runner
  Pod under `/genkit/flows/<flow>/credentials/`.
- Creates and owns the `Deployment` and `Service` that expose each flow
  at `POST /<flow-name>` on the configured port.
- Watches every referenced object and triggers a rolling update via a
  content-hash Pod template annotation whenever anything changes.
- Ships a reference runner image (`ghcr.io/xavidop/genkit-runner`) with
  built-in support for Anthropic, OpenAI, Google AI, Vertex AI, AWS
  Bedrock, Azure AI Foundry and Ollama — but you can swap in your own
  runtime by pointing `spec.image` at any image that honors the
  [runtime contract](docs/runtime-contract.md).

The result: a GitOps-friendly way to ship Genkit flows where the unit
of deployment is a YAML file, not a container.

## Architecture

### The big picture (no Kubernetes knowledge required)

You write a short YAML file that says *"I have these prompts, this model,
and these flows"*. You hand it to the cluster. The operator reads it,
packages everything together, starts your Genkit app, and gives you an
HTTP endpoint per flow. That's it.

```mermaid
flowchart LR
    Dev["You<br/>(write YAML)"] -->|1. submit| K8s["Kubernetes cluster"]
    K8s --> Op["genkit-operator<br/>(the brain)"]
    Op -->|2. assembles your app| App["Your Genkit app<br/>(running container)"]
    User["End user / app"] -->|3. HTTP request| App
    App -->|4. calls| LLM[("LLM provider<br/>Anthropic · OpenAI ·<br/>Google · Ollama")]
```

What you provide:

* **Prompts** — the text instructions for the model.
* **Model** — which provider + model name to use (e.g. Claude Opus).
* **PluginConfig** — credentials (an API key) for the provider.
* **Flow / FlowSet** — *"glue these prompts + this model together and serve
  them at an HTTP route"*.

What you get back: a URL per flow (`POST /<flow-name>`) you can `curl` or
call from any client. No Dockerfiles to write, no servers to wire up, no
credential plumbing — the operator does all of that.

### Detailed view (what actually happens inside Kubernetes)

```mermaid
flowchart LR
    User[User / GitOps]

    subgraph CRDs["CRDs (declarative spec)"]
        PC[PluginConfig<br/>+ Secret ref]
        M[Model]
        P[Prompt]
        T[Tool]
        FS[FlowSet / Flow]
    end

    subgraph Operator["genkit-operator (controller-manager)"]
        R[Reconciler<br/>resolve refs · render · SSA]
    end

    subgraph Rendered["Rendered children (SSA, owned)"]
        MAN[ConfigMap: manifest.json]
        CFG[ConfigMaps per flow:<br/>prompts / tools / config.json]
        SEC[(Secret mount:<br/>credentials/)]
        DEP[Deployment + Service]
    end

    subgraph Pod["Runner Pod (cmd/runner, distroless)"]
        RT[genkit-runner<br/>reads manifest +<br/>per-flow config.json]
        REG{plugin registry<br/>anthropic · openai ·<br/>googleai · vertexai · ollama}
        HTTP[POST /flow-a<br/>POST /flow-b ...]
    end

    Client[HTTP client / curl]

    User -->|kubectl apply| CRDs
    CRDs --> R
    R -->|owns| MAN
    R -->|owns| CFG
    R -->|mounts| SEC
    R -->|owns| DEP
    MAN -.->|mounted at /genkit/manifest.json| RT
    CFG -.->|mounted at /genkit/flows/flow/| RT
    SEC -.->|credentialsDir| REG
    RT --> REG
    REG --> HTTP
    DEP --> Pod
    Client -->|POST| HTTP
```

## Getting Started

### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Test it locally on Kind

The fastest way to try the operator end-to-end is against a
[kind](https://kind.sigs.k8s.io/) cluster. The Makefile ships a one-shot
target that builds both images, loads them into kind, installs the CRDs,
and deploys the controller.

```sh
# 1. Create a kind cluster named "genkit" (matches the default KIND_CLUSTER).
kind create cluster --name genkit

# 2. Build manager + runner images, load them into kind, install CRDs, deploy.
make kind-deploy IMG=genkit-operator:dev

# 3. Apply the sample CRs (FlowSet, Model, PluginConfig, Prompt, ...).
kubectl apply -k config/samples/

# 4. Watch the controller and the rendered workloads.
kubectl -n genkit-operator-system logs deploy/genkit-operator-controller-manager -c manager -f
kubectl get flowset,flow,model,pluginconfig,prompt
```

To iterate on changes, re-run `make kind-deploy IMG=genkit-operator:dev`
(the image tag stays the same; the controller deployment is patched in
place). To redeploy only the runner image after editing `cmd/runner`,
run `make runner-kind-load` and then
`kubectl rollout restart deploy/<flowset-name>`.

Override the cluster name with `KIND_CLUSTER=<name>` on any target:

```sh
make kind-deploy IMG=genkit-operator:dev KIND_CLUSTER=my-cluster
```

### To Deploy on the cluster

Released images are published to GHCR by the [`Release` workflow](.github/workflows/release.yml)
on every `v*` tag:

* Manager: `ghcr.io/xavidop/genkit-operator:<tag>` (also `:latest`)
* Runner:  `ghcr.io/xavidop/genkit-runner:<tag>` (also `:latest`)
* Helm chart: `oci://ghcr.io/xavidop/charts/genkit-operator` (version = tag without the leading `v`)

To deploy a published manager image directly:

```sh
make deploy IMG=ghcr.io/xavidop/genkit-operator:v0.1.0
```

To build and push your **own** image instead:

```sh
make docker-build docker-push IMG=<some-registry>/genkit-operator:tag
```

### Build and publish the Genkit runner image

`Flow` and `FlowSet` Pods run the reference runtime built from
`cmd/runner` (`Dockerfile.runner`). The runner image is what loads the
generated `manifest.json` / `config.json`, registers each flow, and
serves `POST /<flow>`. It is independent from the controller image and
has its own Makefile targets so you can iterate on the runtime without
rebuilding the operator.

The runner image tag is controlled by `RUNNER_IMG` (default
`genkit-runner:dev`).

**Build only:**

```sh
make runner-build RUNNER_IMG=<some-registry>/genkit-runner:tag
```

**Build + push to a registry:**

```sh
make runner-build runner-push RUNNER_IMG=<some-registry>/genkit-runner:tag
```

Released runner images are available at `ghcr.io/xavidop/genkit-runner:<tag>`
(also `:latest`). Reference them in `Flow.spec.image` / `FlowSet.spec.image`
to avoid building locally.

**Build + load into a local kind cluster (no registry required):**

```sh
make runner-kind-load RUNNER_IMG=genkit-runner:dev KIND_CLUSTER=genkit
```

**Use the image in a Flow / FlowSet:** set `spec.image` to the same
value you passed as `RUNNER_IMG`:

```yaml
apiVersion: genkit.dev/v1alpha1
kind: FlowSet
metadata:
  name: greeting-suite
spec:
  image: genkit-runner:dev    # must match RUNNER_IMG
  # ... flows, env, etc.
```

**Pick up code changes:** because Docker may cache stale layers when
only Go source files change, rebuild with `--no-cache` and restart the
workload:

```sh
docker build --no-cache -t genkit-runner:dev -f Dockerfile.runner .
kind load docker-image genkit-runner:dev --name genkit
kubectl rollout restart deploy/<flowset-or-flow-name>
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
# Released image
make deploy IMG=ghcr.io/xavidop/genkit-operator:v0.1.0

# Or your own build
make deploy IMG=<some-registry>/genkit-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

The `Release` workflow attaches a ready-to-use `install.yaml` (pinned to the
released manager image) to every GitHub Release. Install it with:

```sh
kubectl apply -f https://github.com/xavidop/genkit-operator/releases/download/v0.1.0/install.yaml
```

To regenerate it locally against a custom image:

```sh
make build-installer IMG=<some-registry>/genkit-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

### By providing a Helm Chart

The chart is published as an OCI artifact on every release. Install it with:

```sh
helm install genkit-operator \
  oci://ghcr.io/xavidop/charts/genkit-operator \
  --version 0.1.0 \
  --namespace genkit-operator-system --create-namespace
```

The chart is stamped with the matching manager image
(`ghcr.io/xavidop/genkit-operator:<tag>`) at release time, so no extra
`--set` flags are required.

To regenerate the chart sources after changing the project:

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

A chart was generated under `dist/chart` and is what the release workflow
packages and pushes.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

Contributions are very welcome — bug reports, feature requests, docs
improvements and code changes alike. The typical workflow is:

1. **Open an issue first** for anything non-trivial (new CRD field,
   behavior change, new plugin) so we can agree on the design before
   you spend time on a PR.
2. **Fork and branch** off `main`. Keep PRs focused: one logical change
   per PR makes review much easier.
3. **Run the local checks** before pushing:
   ```sh
   make lint-fix     # auto-fix style
   make test         # unit + envtest suite
   make manifests generate  # only if you touched *_types.go or markers
   ```
4. **Test against a real cluster** when changing controller behavior:
   ```sh
   kind create cluster --name genkit
   make kind-deploy IMG=genkit-operator:dev
   kubectl apply -k config/samples/
   ```
5. **Update docs** under `docs/` and the [Astro site](website/) when
   you add user-facing features, and add a `CHANGELOG.md` entry under
   the `Unreleased` section.
6. **Follow the conventions** documented in [AGENTS.md](AGENTS.md) —
   never edit auto-generated files (`zz_generated.*`,
   `config/crd/bases/*`, `config/rbac/role.yaml`,
   `config/webhook/manifests.yaml`, `PROJECT`) by hand; regenerate them
   with `make manifests generate` instead.

By contributing you agree that your contributions are licensed under
the Apache License 2.0.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026 Xavier Portilla Edo.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

