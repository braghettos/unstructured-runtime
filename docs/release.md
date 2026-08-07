---
type: Runbook
title: unstructured-runtime — release
description: How a release actually ships — a tag-only Go library on vX.Y.Z tags, no OCI artifacts, no release workflow.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [release, tags, go-module]
timestamp: 2026-08-07T00:00:00Z
---

# Release

This repo is a **tag-only Go library**. Reality, derived from the repo's
workflows and existing tags:

- **Artifact**: the Go module `github.com/krateo-platformops/unstructured-runtime`,
  fetched by consumers straight from the git tag via the Go module proxy.
  There is **no image, no chart, no OCI artifact** — unlike the platform's
  component repos, nothing here runs `release-oci.yaml`.
- **Tag format**: `vX.Y.Z` **with the `v` prefix** (Go modules require it;
  latest: `v1.4.0`). This deliberately differs from the org's chart/image
  repos, whose tags carry no prefix.
- **No release workflow and no GitHub Releases**: the only CI is
  [.github/workflows/test.yaml](../.github/workflows/test.yaml) —
  `go test -race` with coverage upload to Codecov on every push and PR, plus
  the shared docs linter. Pushing a tag builds nothing; the tag itself is the
  release.

## Runbook

1. Make sure `main` is green (`Test and coverage` workflow) and the working
   tree is what you want to ship: `go build ./... && go test -race ./...`.
2. Tag and push:

   ```sh
   git tag v1.x.y
   git push origin v1.x.y
   ```

3. Verify the module proxy serves it:

   ```sh
   GOPROXY=proxy.golang.org go list -m github.com/krateo-platformops/unstructured-runtime@v1.x.y
   ```

4. Bump consumers — `composition-dynamic-controller` first — with
   `go get github.com/krateo-platformops/unstructured-runtime@v1.x.y`, and
   re-pin [docs/llms.txt](./llms.txt) to the new tag.

Breaking changes to the exported surface require a new major
(`/v2` module path); within `v1` the surface is append-only.
