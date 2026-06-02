// Replaces `{{LATEST_VERSION}}` and `{{LATEST_TAG}}` placeholders inside
// markdown text and inline-code / code-block nodes with the values resolved
// in `latest-version.mjs`.
//
// This runs at build time so the rendered HTML always points at the most
// recent released version without needing client-side JS.

import { version, tag } from "./latest-version.mjs";

const REPLACEMENTS = {
  "{{LATEST_VERSION}}": version,
  "{{LATEST_TAG}}": tag,
};

const PATTERN = /\{\{LATEST_(VERSION|TAG)\}\}/g;

function apply(value) {
  return value.replace(
    PATTERN,
    (match) => REPLACEMENTS[match] ?? match,
  );
}

export default function remarkLatestVersion() {
  return (tree) => {
    visit(tree, (node) => {
      if (
        node &&
        typeof node.value === "string" &&
        (node.type === "text" ||
          node.type === "inlineCode" ||
          node.type === "code" ||
          node.type === "html")
      ) {
        node.value = apply(node.value);
      }
    });
  };
}

function visit(node, fn) {
  fn(node);
  if (Array.isArray(node.children)) {
    for (const child of node.children) visit(child, fn);
  }
}
