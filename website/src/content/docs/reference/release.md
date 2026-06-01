---
title: Release process
description: How releases are cut, versioned, and published.
---

Releases are fully automated. There is no manual `git tag`.

## Conventional Commits → semver

The project follows [Conventional
Commits](https://www.conventionalcommits.org/). The release workflow runs
[semantic-release](https://github.com/semantic-release/semantic-release)
on every push to `main` and infers the next version:

| Commit type            | Bump  |
| ---------------------- | ----- |
| `fix:`                 | patch |
| `feat:`                | minor |
| `feat!:` / `BREAKING CHANGE:` footer | major |

If no qualifying commits are present since the last release, the
workflow runs to completion but does not publish anything.

## What gets published

On every release the [`Release`](https://github.com/xavidop/genkit-operator/actions/workflows/release.yml)
workflow:

1. **Tags** the commit (`vX.Y.Z`) and generates release notes from the
   commit history.
2. **Builds container images in parallel**, one job per
   `(image, platform)`:
   - `manager` on `linux/amd64` (Ubuntu x64 runner)
   - `manager` on `linux/arm64` (Ubuntu arm64 runner)
   - `runner` on `linux/amd64`
   - `runner` on `linux/arm64`
   Each job pushes by digest (no tag) so the four uploads happen
   concurrently.
3. **Merges the digests** into a multi-arch manifest list per image
   with `docker buildx imagetools create`, producing:
   - `ghcr.io/xavidop/genkit-operator:vX.Y.Z` and `:latest`
   - `ghcr.io/xavidop/genkit-runner:vX.Y.Z` and `:latest`
4. **Packages the Helm chart**, stamps it with the version and the
   manager image tag, and pushes it as an OCI artifact:
   - `oci://ghcr.io/xavidop/charts/genkit-operator` (version
     `X.Y.Z`).
5. **Builds `install.yaml`** with `make build-installer` pinned to the
   released manager image.
6. **Attaches** `install.yaml` and the chart `.tgz` to the GitHub
   Release semantic-release created.

## Why per-platform parallel jobs?

QEMU emulation of `linux/arm64` on an `amd64` runner is slow — every
Go build of CGO-less binaries still has to round-trip through user-mode
emulation. Running each platform natively (Ubuntu arm64 runner for
`linux/arm64`, Ubuntu x64 for `linux/amd64`) cuts release time
roughly in half and avoids the QEMU overhead entirely.

## Files

- [.github/workflows/release.yml](https://github.com/xavidop/genkit-operator/blob/main/.github/workflows/release.yml)
- [.releaserc.json](https://github.com/xavidop/genkit-operator/blob/main/.releaserc.json)
- [CHANGELOG.md](https://github.com/xavidop/genkit-operator/blob/main/CHANGELOG.md) (auto-generated)
