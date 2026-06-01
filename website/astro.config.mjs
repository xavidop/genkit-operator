import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://xavidop.github.io",
  base: "/genkit-operator",
  integrations: [
    starlight({
      title: "Genkit Operator",
      description:
        "A Kubernetes operator that turns YAML into production-ready Genkit (Go) HTTP endpoints — no Dockerfiles, no servers, no credential plumbing.",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/xavidop/genkit-operator",
        },
      ],
      editLink: {
        baseUrl:
          "https://github.com/xavidop/genkit-operator/edit/main/website/",
      },
      sidebar: [
        {
          label: "Getting Started",
          items: [
            { label: "Why Genkit Operator?", slug: "why" },
            { label: "Quickstart", slug: "quickstart" },
            { label: "Install", slug: "install" },
          ],
        },
        {
          label: "Concepts",
          items: [
            { label: "Architecture", slug: "architecture" },
            { label: "Custom Resources", slug: "crds" },
            { label: "Runtime contract", slug: "runtime-contract" },
          ],
        },
        {
          label: "Guides",
          items: [
            { label: "Deploy a Flow", slug: "guides/deploy-flow" },
            { label: "FlowSet (multi-flow)", slug: "guides/flowset" },
            { label: "Build a custom runner", slug: "guides/custom-runner" },
          ],
        },
        {
          label: "Plugins",
          items: [
            { label: "Overview", slug: "plugins/overview" },
            { label: "Anthropic", slug: "plugins/anthropic" },
            { label: "OpenAI", slug: "plugins/openai" },
            { label: "Google AI (Gemini)", slug: "plugins/googleai" },
            { label: "Vertex AI", slug: "plugins/vertexai" },
            { label: "AWS Bedrock", slug: "plugins/bedrock" },
            { label: "Ollama", slug: "plugins/ollama" },
          ],
        },
        {
          label: "Samples",
          items: [{ label: "Sample CRs", slug: "samples" }],
        },
        {
          label: "Reference",
          items: [
            { label: "Release process", slug: "reference/release" },
            { label: "Contributing", slug: "reference/contributing" },
          ],
        },
      ],
    }),
  ],
});
