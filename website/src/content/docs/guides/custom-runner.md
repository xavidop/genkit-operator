---
title: Build a custom runner
description: How to build a Genkit-based runner that consumes the operator's mounted CRD content.
---

The runner is the container image you set in `Flow.spec.image` or
`FlowSet.spec.image`. Everything else — which prompts to register,
which tools, which model, which credentials — comes from the CRDs at
runtime, mounted into the container as plain files by the operator.

So a custom runner is essentially a small program that:

1. **Reads** `/genkit/manifest.json` (multi-flow) or
   `/genkit/config.json` (single-flow) to discover its layout.
2. **Loads** prompts from `/genkit/prompts/*.prompt` (or
   `/genkit/flows/<flow>/prompts/*.prompt`) into a
   [Genkit](https://genkit.dev/) instance.
3. **Loads** tools from `/genkit/tools/*.json` and registers each one
   with Genkit.
4. **Reads** credentials from env vars (single-flow) or from
   `/genkit/flows/<flow>/credentials/<KEY>` files (multi-flow).
5. **Serves** `POST /<flowName>` and invokes the entrypoint prompt.

The reference Go runner under
[`cmd/runner`](https://github.com/xavidop/genkit-operator/tree/main/cmd/runner)
does exactly this in ~400 lines and supports every plugin the
operator ships with. Read the full
[runtime contract](/runtime-contract/) before
implementing your own.

## What is mounted, exactly

**Single-flow (`Flow` CR):**

```
/genkit/
├── prompts/<name>.prompt   # raw Dotprompt content
├── tools/<name>.json       # { definition, implementation }
└── config.json             # { defaultModel, plugin }
```
Credentials arrive as **env vars** via `envFrom` on the Secret.

**Multi-flow (`FlowSet` CR):**

```
/genkit/
├── manifest.json           # { flows: [{ name, entrypoint, dir }] }
└── flows/<flowName>/
    ├── prompts/<name>.prompt
    ├── tools/<name>.json
    ├── config.json         # + credentialsDir + credentialKeys
    └── credentials/<KEY>   # one file per secret key
```

The runner must auto-detect: if `/genkit/manifest.json` exists, use
multi-flow; otherwise fall back to single-flow.

## Implementing the contract

Below is the same skeleton — _detect layout → load config → load
prompts → load tools → serve POST_ — in each SDK.

### Genkit Go (reference)

Genkit Go ships first-class Dotprompt loading via
[`genkit.WithPromptDir`](https://genkit.dev/go/docs/dotprompt/) plus
[`genkit.LookupPrompt`](https://pkg.go.dev/github.com/firebase/genkit/go/genkit#LookupPrompt),
which is why it's the reference runner. The shape:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "path/filepath"

    "github.com/firebase/genkit/go/ai"
    "github.com/firebase/genkit/go/genkit"
    "github.com/firebase/genkit/go/plugins/googlegenai"
)

type runtimeConfig struct {
    DefaultModel struct {
        Provider string `json:"provider"`
        Model    string `json:"model"`
    } `json:"defaultModel"`
    Plugin struct {
        Type string `json:"type"`
    } `json:"plugin"`
}

type manifest struct {
    Flows []struct {
        Name, Entrypoint, Dir string
    } `json:"flows"`
}

func main() {
    ctx := context.Background()
    mux := http.NewServeMux()

    if _, err := os.Stat("/genkit/manifest.json"); err == nil {
        b, _ := os.ReadFile("/genkit/manifest.json")
        var m manifest
        _ = json.Unmarshal(b, &m)
        for _, f := range m.Flows {
            g := initFlow(ctx, f.Dir)
            p := genkit.LookupPrompt(g, f.Entrypoint)
            mux.HandleFunc("POST /"+f.Name, handler(p))
        }
    } else {
        g := initFlow(ctx, "/genkit")
        // single-flow: one HTTP route per .prompt file
        matches, _ := filepath.Glob("/genkit/prompts/*.prompt")
        for _, m := range matches {
            name := filepath.Base(m[:len(m)-len(".prompt")])
            p := genkit.LookupPrompt(g, name)
            mux.HandleFunc("POST /"+name, handler(p))
        }
    }
    log.Fatal(http.ListenAndServe(":8080", mux))
}

func initFlow(ctx context.Context, dir string) *genkit.Genkit {
    b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
    var cfg runtimeConfig
    _ = json.Unmarshal(b, &cfg)
    return genkit.Init(ctx,
        genkit.WithPlugins(&googlegenai.GoogleAI{}),
        genkit.WithDefaultModel(cfg.DefaultModel.Provider+"/"+cfg.DefaultModel.Model),
        genkit.WithPromptDir(filepath.Join(dir, "prompts")),
    )
}

func handler(p ai.Prompt) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var in map[string]any
        _ = json.NewDecoder(r.Body).Decode(&in)
        resp, err := p.Execute(r.Context(), ai.WithInput(in))
        if err != nil { http.Error(w, err.Error(), 500); return }
        _, _ = w.Write([]byte(resp.Text()))
    }
}
```

Real production concerns the reference runner adds: a per-plugin
registry that branches on `cfg.Plugin.Type`, credential file loading
for FlowSet pods (`os.ReadFile(filepath.Join(cfg.CredentialsDir, key))`),
tool registration from `/genkit/tools/*.json`, and a
[staging step](https://github.com/xavidop/genkit-operator/blob/main/cmd/runner/main.go)
to dodge double-registration from Kubernetes' ConfigMap symlink
layout. See [`cmd/runner/main.go`](https://github.com/xavidop/genkit-operator/blob/main/cmd/runner/main.go)
and [`cmd/runner/plugins.go`](https://github.com/xavidop/genkit-operator/blob/main/cmd/runner/plugins.go).

### Genkit JS / TypeScript

Genkit JS has the most complete Dotprompt support: pass
`promptDir` to `genkit({ ... })` and look up by name with
`ai.prompt(name)`
([Dotprompt docs](https://genkit.dev/docs/dotprompt/)). Pair with
[`@genkit-ai/express`](https://genkit.dev/docs/deploy-node/)'s
`startFlowServer` to serve over HTTP.

```ts
import { readFileSync, existsSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { genkit, z } from "genkit";
import { googleAI } from "@genkit-ai/google-genai";
import { startFlowServer } from "@genkit-ai/express";

type Cfg = {
  defaultModel: { provider: string; model: string };
  plugin: { type: string };
  credentialsDir?: string;
  credentialKeys?: string[];
};

function loadCredentials(cfg: Cfg) {
  if (!cfg.credentialsDir) return;
  for (const key of cfg.credentialKeys ?? []) {
    const p = join(cfg.credentialsDir, key);
    if (existsSync(p)) process.env[key] = readFileSync(p, "utf8").trim();
  }
}

function buildFlowFromDir(dir: string, name: string) {
  const cfg: Cfg = JSON.parse(readFileSync(join(dir, "config.json"), "utf8"));
  loadCredentials(cfg);

  const ai = genkit({
    plugins: [googleAI()], // pick plugin from cfg.plugin.type in production
    model: `${cfg.defaultModel.provider}/${cfg.defaultModel.model}`,
    promptDir: join(dir, "prompts"),
  });

  // Each .prompt becomes a callable. The flow we expose is the entrypoint.
  const entryFile = readdirSync(join(dir, "prompts"))
    .find((f) => f.endsWith(".prompt"))!;
  const entryName = entryFile.replace(/\.prompt$/, "");
  const callable = ai.prompt(entryName);

  return ai.defineFlow(
    { name, inputSchema: z.any(), outputSchema: z.any() },
    async (input) => (await callable(input)).output ?? (await callable(input)).text,
  );
}

const flows = existsSync("/genkit/manifest.json")
  ? (JSON.parse(readFileSync("/genkit/manifest.json", "utf8"))
      .flows as Array<{ name: string; entrypoint: string; dir: string }>)
      .map((f) => buildFlowFromDir(f.dir, f.name))
  : [buildFlowFromDir("/genkit", "default")];

startFlowServer({ flows });
```

`startFlowServer` exposes each flow as `POST /<name>` with body
`{"data": <input>}`. Default port is `3400` — override via `PORT`.

> Note: JS dispatches per-plugin (`googleAI`, `openAI`, …) at module
> import time. If you need to support multiple providers in one image
> you'll branch on `cfg.plugin.type` and dynamically import the right
> plugin package.

### Genkit Python

Python's Dotprompt support is lighter than JS/Go's; idiomatic Python
runners typically use `@ai.flow()` and call `ai.generate(...)`
directly. You still read the same files — you just translate prompt
content into `generate` calls instead of using a registry lookup.

```python
import json
import os
from pathlib import Path

from genkit import Genkit
from genkit.plugins.google_genai import GoogleAI

def load_credentials(cfg: dict) -> None:
    cred_dir = cfg.get("credentialsDir")
    if not cred_dir:
        return
    for key in cfg.get("credentialKeys", []):
        f = Path(cred_dir) / key
        if f.exists():
            os.environ[key] = f.read_text().strip()

def build_flow(ai: Genkit, name: str, dir_: Path, entrypoint: str):
    prompt_body = (dir_ / "prompts" / f"{entrypoint}.prompt").read_text()

    @ai.flow(name=name)
    async def _flow(input_data: dict) -> str:
        # Render Dotprompt YAML+Handlebars yourself, or pass the body
        # straight through. Production runners typically use the
        # `dotpromptz` package to render templates from the .prompt body.
        result = await ai.generate(prompt=prompt_body)
        return result.text

    return _flow

def main():
    if Path("/genkit/manifest.json").exists():
        manifest = json.loads(Path("/genkit/manifest.json").read_text())
        for fm in manifest["flows"]:
            cfg = json.loads((Path(fm["dir"]) / "config.json").read_text())
            load_credentials(cfg)
            ai = Genkit(
                plugins=[GoogleAI()],
                model=f'{cfg["defaultModel"]["provider"]}/{cfg["defaultModel"]["model"]}',
            )
            build_flow(ai, fm["name"], Path(fm["dir"]), fm["entrypoint"])
    else:
        cfg = json.loads(Path("/genkit/config.json").read_text())
        ai = Genkit(
            plugins=[GoogleAI()],
            model=f'{cfg["defaultModel"]["provider"]}/{cfg["defaultModel"]["model"]}',
        )
        # one flow per *.prompt file
        for p in (Path("/genkit") / "prompts").glob("*.prompt"):
            build_flow(ai, p.stem, Path("/genkit"), p.stem)

    # Expose flows over HTTP with the framework of your choice (FastAPI,
    # Flask, etc.) and listen on $PORT.

main()
```

See the [Python getting-started](https://genkit.dev/python/docs/get-started/)
guide for the supported plugin set and the Genkit-recommended HTTP
deployment patterns.

### Genkit Java

The unofficial [`genkit-java`](https://genkit-ai.github.io/genkit-java/)
distribution exposes flows over HTTP via its
[Jetty plugin](https://genkit-ai.github.io/genkit-java/plugins/jetty/):
defined flows are auto-served at `POST /api/flows/<flowName>`. Read
the mounted files in `main()` the same way:

```java
import com.google.gson.Gson;
import com.google.gson.JsonObject;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;

import com.google.genkit.Genkit;
import com.google.genkit.ai.GenerateOptions;
import com.google.genkit.plugins.googlegenai.GoogleGenAIPlugin;
import com.google.genkit.plugins.jetty.JettyPlugin;
import com.google.genkit.plugins.jetty.JettyPluginOptions;

public class Main {
    public static void main(String[] args) throws Exception {
        JettyPlugin jetty = new JettyPlugin(
            JettyPluginOptions.builder().port(8080).build());
        Genkit genkit = Genkit.builder()
            .plugin(GoogleGenAIPlugin.create())
            .plugin(jetty)
            .build();

        Gson gson = new Gson();
        Path manifestPath = Paths.get("/genkit/manifest.json");

        if (Files.exists(manifestPath)) {
            JsonObject m = gson.fromJson(Files.readString(manifestPath), JsonObject.class);
            for (var el : m.getAsJsonArray("flows")) {
                JsonObject f = el.getAsJsonObject();
                String name = f.get("name").getAsString();
                Path dir = Paths.get(f.get("dir").getAsString());
                String entry = f.get("entrypoint").getAsString();
                registerFlow(genkit, name, dir, entry);
            }
        } else {
            Path dir = Paths.get("/genkit");
            try (var s = Files.list(dir.resolve("prompts"))) {
                s.filter(p -> p.toString().endsWith(".prompt")).forEach(p -> {
                    String n = p.getFileName().toString().replace(".prompt", "");
                    registerFlow(genkit, n, dir, n);
                });
            }
        }
        jetty.start();
    }

    static void registerFlow(Genkit genkit, String name, Path dir, String entry) {
        try {
            String promptBody = Files.readString(dir.resolve("prompts/" + entry + ".prompt"));
            genkit.defineFlow(name, String.class, String.class, (ctx, input) ->
                genkit.generate(GenerateOptions.builder()
                    .model("googleai/gemini-2.0-flash")
                    .prompt(promptBody) // render the Dotprompt body yourself
                    .build()).getText());
        } catch (Exception e) { throw new RuntimeException(e); }
    }
}
```

Same caveat as Python: the genkit-java SDK doesn't ship a Dotprompt
file loader — you read the `.prompt` body and pass it to
`GenerateOptions.builder().prompt(...)`, doing your own
frontmatter/Handlebars rendering if needed.

## Tools mount (`/genkit/tools/*.json`)

Every tool referenced by your `Flow`/`FlowSet` lands as a JSON file:

```json
{
  "definition": { "name": "...", "description": "...", "inputSchema": {...} },
  "implementation": {
    "http": { "url": "https://...", "method": "POST" }
  }
}
```

Your runner registers each one with Genkit (`ai.defineTool` in JS,
`genkit.DefineTool` in Go, etc.) and dispatches calls based on
`implementation.http` / `implementation.flowRef`. The Go runner's
implementation is the easiest reference.

## Dockerfile

Whatever the language, the image just needs to listen on the port set
by `Flow.spec.port` / `FlowSet.spec.port` (default `8080`):

```dockerfile
# Go
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/runner ./cmd/runner
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/runner /runner
EXPOSE 8080
ENTRYPOINT ["/runner"]
```

```dockerfile
# Node
FROM node:22-slim
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY . .
USER 65532:65532
EXPOSE 8080
CMD ["node", "lib/runner.js"]
```

```dockerfile
# Python
FROM python:3.13-slim
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN pip install --no-cache-dir uv && uv sync --frozen
COPY runner.py .
USER 65532:65532
EXPOSE 8080
CMD ["uv", "run", "runner.py"]
```

```dockerfile
# Java
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY target/runner.jar .
USER 65532:65532
EXPOSE 8080
CMD ["java", "-jar", "runner.jar"]
```

Build multi-arch and push:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/yourorg/my-runner:1.0.0 --push .
```

## Point your Flow at it

```yaml
apiVersion: genkit.dev/v1alpha1
kind: Flow
metadata:
  name: greeter
spec:
  image: ghcr.io/yourorg/my-runner:1.0.0
  # ... rest unchanged ...
```

The operator doesn't care which language or framework — only that the
container honours the file/env contract above.

## Tips

- **Test the contract locally**, no operator needed: build a `/genkit`
  directory on disk, run the binary against it.
- **Use the Go runner as your spec.** Code is ~400 lines and covers
  every plugin shipped with the operator.
- **Health probes:** the operator wires `livenessProbe`/`readinessProbe`
  to `/healthz` and `/readyz`. Return cheap `200 OK`s.
- **Hot reload is free.** Any change to a referenced CR bumps
  `Flow.status.contentHash`, which bumps the Pod annotation, which
  triggers a rolling restart — the runner never has to watch files.

## Why build your own

- Your team already ships in Genkit JS / Python / Java and wants one
  stack.
- You need a Genkit plugin the reference runner doesn't bundle.
- You want custom middleware: caching, auth, rate limiting, custom
  observability.
- Compliance — fully-audited image with your security baseline baked in.
