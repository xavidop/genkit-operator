// Resolves the version that should be referenced in the docs at build time.
//
// Resolution order:
//   1. `LATEST_VERSION` env var (e.g. set in CI right before `astro build`).
//      May be either `1.2.3` or `v1.2.3`.
//   2. GitHub Releases API (`/repos/<owner>/<repo>/releases/latest`).
//      Only attempted when `FETCH_LATEST_VERSION` is truthy so that local
//      builds remain hermetic and offline-friendly.
//   3. Fallback to `latest`, which still produces valid container image
//      tags and a sensible "use the most recent chart" hint for Helm.
//
// Exposes two strings:
//   - `version`: numeric form, no leading `v` (e.g. `1.2.3`). Used for
//     `helm install --version`.
//   - `tag`: git tag form, with leading `v` (e.g. `v1.2.3`). Used for
//     GitHub release URLs and container image tags.

const REPO = process.env.GENKIT_OPERATOR_REPO ?? "xavidop/genkit-operator";

function normalize(raw) {
  if (!raw) return null;
  const trimmed = String(raw).trim();
  if (!trimmed) return null;
  const version = trimmed.replace(/^v/, "");
  return { version, tag: `v${version}` };
}

async function fromGithub() {
  if (!process.env.FETCH_LATEST_VERSION) return null;
  try {
    const res = await fetch(
      `https://api.github.com/repos/${REPO}/releases/latest`,
      {
        headers: {
          accept: "application/vnd.github+json",
          "user-agent": "genkit-operator-docs-build",
          ...(process.env.GITHUB_TOKEN
            ? { authorization: `Bearer ${process.env.GITHUB_TOKEN}` }
            : {}),
        },
      },
    );
    if (!res.ok) return null;
    const data = await res.json();
    return normalize(data?.tag_name);
  } catch {
    return null;
  }
}

const resolved =
  normalize(process.env.LATEST_VERSION) ??
  (await fromGithub()) ?? { version: "latest", tag: "latest" };

export const version = resolved.version;
export const tag = resolved.tag;
