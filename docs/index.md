---
type: Library
title: unstructured-runtime — index
description: The map of the unstructured-runtime doc bundle — Krateo's managed-reconciler framework for dynamic, unstructured Kubernetes controllers.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [library, controller-runtime, managed-reconciler, unstructured]
timestamp: 2026-08-07T00:00:00Z
---

# unstructured-runtime

unstructured-runtime is **Krateo's own composition runtime**: a Go library that
turns an `ExternalClient` (Observe / Create / Update / Delete) into a complete
Kubernetes controller for any GroupVersionResource, using dynamic clients and
`unstructured.Unstructured` objects only — so it can reconcile CRDs that are
generated at runtime (Krateo compositions) and have no compiled Go types. The
library owns the reconcile loop, the dedup'd priority queue with rate-limited
retry, the `krateo.io/external-create-*` annotation handshake with
incomplete-create recovery, finalizers, graceful drain, and OTel telemetry.
Its primary consumer is Krateo's `composition-dynamic-controller`, so every
composition on the platform reconciles through this code.

## The bundle (start here)

- [overview](./overview.md) — how it works: event intake and classification,
  the priority queue, the worker reconcile loop, the external-create handshake,
  incomplete-create recovery (2-minute grace + confirm-the-negative), the
  deliberate fail-on-409 pending write, and why `ExternalClient.Observe` must
  be side-effect-free.
- [usage](./usage.md) — `go get` + the minimal code from zero to a running
  controller; the `ExternalClient` contract.
- [configuration](./configuration.md) — the whole config surface: builder
  options, `controller.Options`, `telemetry.Config`, and the environment
  variables the library reads.
- [api](./api.md) — the exported Go API surface, package by package.
- [examples](./examples.md) — the runnable examples under `examples/`.
- [release](./release.md) — how a release ships: tag-only Go library,
  `vX.Y.Z` tags, no OCI artifacts.
- [log](./log.md) — curated history.
- [llms.txt](./llms.txt) — the version-pinned agent index of this bundle.
