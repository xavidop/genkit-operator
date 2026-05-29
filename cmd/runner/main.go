/*
Copyright 2026 Xavier Portilla Edo.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Command runner is a reference Genkit runtime that satisfies the
// genkit-operator runtime contract documented in docs/runtime-contract.md.
//
// It supports BOTH layouts:
//
//   - Single-flow (Flow CR): reads /genkit/config.json + /genkit/prompts/*
//     and exposes POST /<prompt-name>.
//   - Multi-flow (FlowSet CR): reads /genkit/manifest.json, then per flow
//     /genkit/flows/<flow>/{config.json,prompts/*,credentials/} and
//     exposes POST /<flow-name> using the flow's entrypoint prompt.
//
// The layout is auto-detected: if /genkit/manifest.json exists, multi-flow
// mode is used; otherwise the runner falls back to single-flow mode.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
)

const (
	mountRoot    = "/genkit"
	promptsDir   = mountRoot + "/prompts"
	configFile   = mountRoot + "/config.json"
	manifestFile = mountRoot + "/manifest.json"
	flowsDir     = mountRoot + "/flows"
	listenAddr   = ":8080"
	// stagedRoot is a clean tmpfs root the runner copies prompts into
	// before handing the path to genkit. Kubernetes ConfigMap volumes expose
	// a `..data` symlink layout that causes genkit's LoadPromptDir to
	// register each prompt twice; staging avoids that.
	stagedRoot = "/tmp/genkit-staged"
)

// runtimeConfig is the schema of single-flow /genkit/config.json AND of
// each per-flow /genkit/flows/<flow>/config.json (which additionally sets
// CredentialsDir + CredentialKeys).
type runtimeConfig struct {
	DefaultModel struct {
		Provider string          `json:"provider"`
		Model    string          `json:"model"`
		Config   json.RawMessage `json:"config,omitempty"`
	} `json:"defaultModel"`
	Plugin struct {
		Type        string          `json:"type"`
		Region      string          `json:"region,omitempty"`
		ExtraConfig json.RawMessage `json:"extraConfig,omitempty"`
	} `json:"plugin"`
	CredentialsDir string   `json:"credentialsDir,omitempty"`
	CredentialKeys []string `json:"credentialKeys,omitempty"`
}

// flowSetManifest is the schema of /genkit/manifest.json written by the
// FlowSet controller.
type flowSetManifest struct {
	Flows []struct {
		Name       string `json:"name"`
		Entrypoint string `json:"entrypoint"`
		Dir        string `json:"dir"`
	} `json:"flows"`
}

func main() {
	ctx := context.Background()

	mux := http.NewServeMux()
	var routes []string

	if _, err := os.Stat(manifestFile); err == nil {
		routes = runMultiFlow(ctx, mux)
	} else if os.IsNotExist(err) {
		routes = runSingleFlow(ctx, mux)
	} else {
		log.Fatalf("stat manifest: %v", err)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"routes": routes})
	})

	log.Printf("listening on %s; routes=%v", listenAddr, routes)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// runSingleFlow handles the legacy single-Flow runtime layout. It returns
// the list of registered HTTP routes.
func runSingleFlow(ctx context.Context, mux *http.ServeMux) []string {
	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	plugin, err := buildPlugin(cfg, "")
	if err != nil {
		log.Fatalf("plugin: %v", err)
	}
	staged := filepath.Join(stagedRoot, "default", "prompts")
	if err := stagePrompts(promptsDir, staged); err != nil {
		log.Fatalf("stage prompts: %v", err)
	}
	g := initGenkit(ctx, plugin, cfg, staged)
	log.Printf("genkit initialized (single-flow); plugin=%s", cfg.Plugin.Type)
	return registerRoutes(g, mux, staged, "")
}

// runMultiFlow handles the FlowSet runtime layout. Each flow becomes a
// separate genkit registry, and HTTP routing is POST /<flow-name> (calling
// that flow's entrypoint prompt).
func runMultiFlow(ctx context.Context, mux *http.ServeMux) []string {
	var manifest flowSetManifest
	b, err := os.ReadFile(manifestFile)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		log.Fatalf("parse manifest: %v", err)
	}
	log.Printf("genkit initialized (multi-flow); flows=%d", len(manifest.Flows))

	var routes []string
	for _, fm := range manifest.Flows {
		flowName := fm.Name
		cfgPath := filepath.Join(fm.Dir, "config.json")
		cfg, err := loadConfig(cfgPath)
		if err != nil {
			log.Fatalf("flow %q: load config: %v", flowName, err)
		}
		plugin, err := buildPlugin(cfg, cfg.CredentialsDir)
		if err != nil {
			log.Fatalf("flow %q: plugin: %v", flowName, err)
		}
		srcPrompts := filepath.Join(fm.Dir, "prompts")
		staged := filepath.Join(stagedRoot, flowName, "prompts")
		if err := stagePrompts(srcPrompts, staged); err != nil {
			log.Fatalf("flow %q: stage prompts: %v", flowName, err)
		}
		g := initGenkit(ctx, plugin, cfg, staged)

		// In multi-flow mode the only public route per flow is its
		// entrypoint at POST /<flow-name>.
		if fm.Entrypoint == "" {
			log.Printf("WARN: flow %q has no entrypoint; skipping route", flowName)
			continue
		}
		p := genkit.LookupPrompt(g, fm.Entrypoint)
		if p == nil {
			log.Fatalf("flow %q: entrypoint prompt %q not registered", flowName, fm.Entrypoint)
		}
		path := "/" + flowName
		mux.HandleFunc(path, promptHandler(p))
		routes = append(routes, path)
		log.Printf("registered flow %q at POST %s (entrypoint=%s)", flowName, path, fm.Entrypoint)
	}
	return routes
}

func initGenkit(ctx context.Context, plugin api.Plugin, cfg *runtimeConfig, promptDir string) *genkit.Genkit {
	defaultModel := ""
	if cfg.DefaultModel.Provider != "" && cfg.DefaultModel.Model != "" {
		defaultModel = cfg.DefaultModel.Provider + "/" + cfg.DefaultModel.Model
	}
	opts := []genkit.GenkitOption{
		genkit.WithPlugins(plugin),
		genkit.WithPromptDir(promptDir),
	}
	if defaultModel != "" {
		opts = append(opts, genkit.WithDefaultModel(defaultModel))
	}
	return genkit.Init(ctx, opts...)
}

// registerRoutes globs the staged prompts dir and wires one HTTP route per
// prompt under routePrefix. Empty prefix produces POST /<prompt-name>;
// non-empty produces POST /<prefix>/<prompt-name>.
func registerRoutes(g *genkit.Genkit, mux *http.ServeMux, stagedDir, routePrefix string) []string {
	matches, err := filepath.Glob(filepath.Join(stagedDir, "*.prompt"))
	if err != nil {
		log.Fatalf("scan prompts: %v", err)
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := strings.TrimSuffix(filepath.Base(m), ".prompt")
		p := genkit.LookupPrompt(g, name)
		if p == nil {
			log.Printf("WARN: prompt %q not registered by genkit; skipping", name)
			continue
		}
		path := "/" + name
		if routePrefix != "" {
			path = "/" + routePrefix + "/" + name
		}
		mux.HandleFunc(path, promptHandler(p))
		names = append(names, path)
		log.Printf("registered prompt route POST %s", path)
	}
	return names
}

// buildPlugin lives in plugins.go: it dispatches to the per-provider
// builder registered in pluginRegistry. To add a new provider, edit
// plugins.go only. Generic credential resolution happens inside the
// plugin builders via readCredential / requireCredential.

func loadConfig(path string) (*runtimeConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c runtimeConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, nil
}

func promptHandler(p ai.Prompt) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input map[string]any
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		resp, err := p.Execute(r.Context(), ai.WithInput(input))
		if err != nil {
			log.Printf("prompt execute error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(resp.Text()))
	}
}

// stagePrompts copies every *.prompt file from src into a freshly created
// dst directory. This avoids genkit's LoadPromptDir double-registering
// each prompt because Kubernetes ConfigMap volumes expose a `..data`
// symlinked layout that gets walked twice.
func stagePrompts(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".prompt") {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// flowsDir is referenced for documentation parity; not currently used by
// the runner outside the manifest-driven multi-flow path.
var _ = flowsDir
