---
type: ExampleIndex
title: unstructured-runtime — examples
description: Index of the runnable examples under examples/.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [examples]
timestamp: 2026-08-07T00:00:00Z
---

# Examples

- [examples/quickstart](../examples/quickstart/README.md) — compilable
  `main.go`: a no-op `ExternalClient` reconciling a GVR out-of-cluster via the
  builder (kubeconfig + graceful shutdown). Builds with plain
  `go build ./...`.
- [examples/integration](../examples/integration/README.md) — end-to-end
  integration test (`-tags integration`): spins up a local kind cluster with
  the e2e-framework, installs a sample CRD, runs a controller and asserts the
  Observe/Create/Delete flow.
