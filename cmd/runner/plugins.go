/*
Copyright 2026 Xavier Portilla Edo.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/plugins/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	bedrock "github.com/xavidop/genkit-aws-bedrock-go"
	azureaifoundry "github.com/xavidop/genkit-azure-foundry-go"
)

// pluginBuilder constructs a genkit-go api.Plugin from the operator's
// runtimeConfig. credentialsDir is the directory where the FlowSet runtime
// has mounted the referenced Secret (single-Flow runtime passes "" because
// credentials are exposed via environment variables). credentialKeys is
// the user-declared list from PluginConfig.spec.credentialKeys; empty
// means "fall back to this plugin's defaults".
type pluginBuilder func(cfg *runtimeConfig, credentialsDir string, credentialKeys []string) (api.Plugin, error)

// pluginRegistry maps PluginConfig.spec.type (lowercased) to a builder.
//
// Adding a new provider is a one-liner: import its package, write the
// builder closure, register it here. No other file needs to change.
var pluginRegistry = map[string]pluginBuilder{
	"anthropic":      buildAnthropic,
	"openai":         buildOpenAI,
	"googleai":       buildGoogleAI,
	"vertexai":       buildVertexAI,
	"ollama":         buildOllama,
	"bedrock":        buildBedrock,
	"azureaifoundry": buildAzureAIFoundry,
}

// defaultCredentialKeys is the conventional Secret-key fallback per
// plugin type. Used only when PluginConfig.spec.credentialKeys is empty.
var defaultCredentialKeys = map[string][]string{
	"anthropic":      {"ANTHROPIC_API_KEY"},
	"openai":         {"OPENAI_API_KEY"},
	"googleai":       {"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	"vertexai":       {"GOOGLE_APPLICATION_CREDENTIALS"},
	"ollama":         nil,
	"bedrock":        {"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"},
	"azureaifoundry": {"AZURE_OPENAI_API_KEY"},
}

func buildPlugin(cfg *runtimeConfig, credentialsDir string) (api.Plugin, error) {
	name := strings.ToLower(cfg.Plugin.Type)
	build, ok := pluginRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported plugin %q (runner supports: %s)",
			cfg.Plugin.Type, strings.Join(registeredPluginNames(), ", "))
	}
	keys := cfg.CredentialKeys
	if len(keys) == 0 {
		keys = defaultCredentialKeys[name]
	}
	return build(cfg, credentialsDir, keys)
}

func registeredPluginNames() []string {
	names := make([]string, 0, len(pluginRegistry))
	for k := range pluginRegistry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// readCredential returns the first credential key that resolves. When
// credentialsDir is non-empty it reads files; otherwise it falls back to
// environment variables. The bool indicates whether anything was found.
func readCredential(credentialsDir string, keys []string) (string, bool) {
	for _, k := range keys {
		if credentialsDir != "" {
			if b, err := os.ReadFile(filepath.Join(credentialsDir, k)); err == nil {
				return strings.TrimSpace(string(b)), true
			}
			continue
		}
		if v := os.Getenv(k); v != "" {
			return v, true
		}
	}
	return "", false
}

// requireCredential is like readCredential but returns an error when no
// key is found, mentioning every candidate the caller tried.
func requireCredential(provider, credentialsDir string, keys []string) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("%s: no credential keys configured", provider)
	}
	v, ok := readCredential(credentialsDir, keys)
	if !ok {
		src := "environment"
		if credentialsDir != "" {
			src = credentialsDir
		}
		return "", fmt.Errorf("%s: none of %v found in %s", provider, keys, src)
	}
	return v, nil
}

func buildAnthropic(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	key, err := requireCredential("anthropic", credentialsDir, keys)
	if err != nil {
		return nil, err
	}
	return &anthropic.Anthropic{APIKey: key}, nil
}

func buildOpenAI(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	key, err := requireCredential("openai", credentialsDir, keys)
	if err != nil {
		return nil, err
	}
	return &openai.OpenAI{APIKey: key}, nil
}

func buildGoogleAI(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	key, err := requireCredential("googleai", credentialsDir, keys)
	if err != nil {
		return nil, err
	}
	return &googlegenai.GoogleAI{APIKey: key}, nil
}

// vertexExtraConfig is the optional shape of plugin.extraConfig for the
// vertexai plugin. Only the JSON fields are read; anything else is
// ignored. Fields take precedence over the runtimeConfig.Plugin.Region
// shortcut for backward compatibility.
type vertexExtraConfig struct {
	ProjectID string `json:"projectId,omitempty"`
	Location  string `json:"location,omitempty"`
}

func buildVertexAI(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	v := &googlegenai.VertexAI{Location: cfg.Plugin.Region}
	if len(cfg.Plugin.ExtraConfig) > 0 {
		var x vertexExtraConfig
		if err := json.Unmarshal(cfg.Plugin.ExtraConfig, &x); err == nil {
			if x.ProjectID != "" {
				v.ProjectID = x.ProjectID
			}
			if x.Location != "" {
				v.Location = x.Location
			}
		}
	}
	// VertexAI uses Google ADC. If GOOGLE_APPLICATION_CREDENTIALS is in the
	// credentialsDir we point the env var at it so the SDK picks it up.
	if credentialsDir != "" {
		for _, k := range keys {
			path := filepath.Join(credentialsDir, k)
			if _, err := os.Stat(path); err == nil {
				_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
				break
			}
		}
	}
	return v, nil
}

// ollamaExtraConfig is the optional shape of plugin.extraConfig for the
// ollama plugin.
type ollamaExtraConfig struct {
	ServerAddress string `json:"serverAddress,omitempty"`
}

func buildOllama(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	o := &ollama.Ollama{}
	if len(cfg.Plugin.ExtraConfig) > 0 {
		var x ollamaExtraConfig
		if err := json.Unmarshal(cfg.Plugin.ExtraConfig, &x); err == nil {
			o.ServerAddress = x.ServerAddress
		}
	}
	if o.ServerAddress == "" {
		return nil, fmt.Errorf("ollama: plugin.extraConfig.serverAddress is required")
	}
	return o, nil
}

// bedrockExtraConfig is the optional shape of plugin.extraConfig for the
// bedrock plugin. Region in extraConfig takes precedence over the
// runtimeConfig.Plugin.Region shortcut.
type bedrockExtraConfig struct {
	Region string `json:"region,omitempty"`
}

func buildBedrock(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	region := cfg.Plugin.Region
	if len(cfg.Plugin.ExtraConfig) > 0 {
		var x bedrockExtraConfig
		if err := json.Unmarshal(cfg.Plugin.ExtraConfig, &x); err == nil && x.Region != "" {
			region = x.Region
		}
	}
	// AWS SDK uses standard env vars + the default credential chain. When
	// credentials are mounted as files (FlowSet layout) load them into the
	// process environment so the SDK picks them up. AWS_SESSION_TOKEN is
	// optional (only used for STS / temporary credentials).
	if credentialsDir != "" {
		for _, k := range keys {
			if b, err := os.ReadFile(filepath.Join(credentialsDir, k)); err == nil {
				_ = os.Setenv(k, strings.TrimSpace(string(b)))
			}
		}
	}
	return &bedrock.Bedrock{Region: region}, nil
}

// azureAIFoundryExtraConfig is the optional shape of plugin.extraConfig
// for the azureaifoundry plugin. Endpoint may also be supplied via the
// AZURE_OPENAI_ENDPOINT credential key (env var or mounted file).
type azureAIFoundryExtraConfig struct {
	Endpoint   string `json:"endpoint,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

func buildAzureAIFoundry(cfg *runtimeConfig, credentialsDir string, keys []string) (api.Plugin, error) {
	var x azureAIFoundryExtraConfig
	if len(cfg.Plugin.ExtraConfig) > 0 {
		_ = json.Unmarshal(cfg.Plugin.ExtraConfig, &x)
	}
	endpoint := x.Endpoint
	if endpoint == "" {
		if v, ok := readCredential(credentialsDir, []string{"AZURE_OPENAI_ENDPOINT"}); ok {
			endpoint = v
		}
	}
	if endpoint == "" {
		return nil, fmt.Errorf("azureaifoundry: endpoint is required " +
			"(set plugin.extraConfig.endpoint or AZURE_OPENAI_ENDPOINT)")
	}
	apiKey, err := requireCredential("azureaifoundry", credentialsDir, keys)
	if err != nil {
		return nil, err
	}
	return &azureaifoundry.AzureAIFoundry{
		Endpoint:   endpoint,
		APIKey:     apiKey,
		APIVersion: x.APIVersion,
	}, nil
}
