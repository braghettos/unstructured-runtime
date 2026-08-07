---
type: Example
title: unstructured-runtime — quickstart example
description: Compilable main.go running a no-op ExternalClient controller against any cluster via the builder.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [example, builder, external-client]
timestamp: 2026-08-07T00:00:00Z
---

# Quickstart

A minimal, compilable controller: a no-op `ExternalClient` (it only logs)
reconciling an arbitrary GVR, built with `pkg/controller/builder`, loading an
out-of-cluster kubeconfig and shutting down gracefully on Ctrl-C. `Observe`
is a pure read, as every implementation's must be
([why](../../docs/overview.md#observe-must-be-side-effect-free)).

## Preconditions

- Go 1.25+
- A kubeconfig (`KUBECONFIG` or `~/.kube/config`) pointing at any cluster
  that serves the GVR you pass — the client only reads and annotates the
  watched CRs, never your workloads.

## Run

```sh
go run ./examples/quickstart -group samples.krateo.io -version v1 -resource samples
```

It compiles as part of the ordinary module build (`go build ./...`).
