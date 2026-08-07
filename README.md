# unstructured-runtime

Krateo's own composition runtime: a managed-reconciler framework for building Kubernetes controllers over dynamic clients and unstructured objects — no generated typed clients required.

[![Go Report Card](https://goreportcard.com/badge/github.com/krateo-platformops/unstructured-runtime)](https://goreportcard.com/report/github.com/krateo-platformops/unstructured-runtime)
[![Test and coverage](https://github.com/krateo-platformops/unstructured-runtime/actions/workflows/test.yaml/badge.svg)](https://github.com/krateo-platformops/unstructured-runtime/actions/workflows/test.yaml)

## What is this

A Go library that turns an `ExternalClient` (four methods: Observe / Create /
Update / Delete) into a full controller for any GroupVersionResource, including
CRDs that only exist at runtime. It provides the reconcile loop, a dedup'd
priority queue with rate-limited retry, the `krateo.io/external-create-*`
annotation handshake with incomplete-create recovery, finalizer handling,
graceful drain on shutdown, and OpenTelemetry metrics/traces/logs. It is the
engine inside Krateo's `composition-dynamic-controller`.
Full picture: [docs/index.md](docs/index.md).

## Install

```sh
go get github.com/krateo-platformops/unstructured-runtime
```

## Configure

See [docs/configuration.md](docs/configuration.md). Most used:

| Setting | Default | Effect |
|---|---|---|
| `builder.WithResyncInterval(d)` | `3m` | informer resync → periodic Observe of every CR |
| `builder.WithMaxRetries(n)` | `5` | retries before a failed event is dropped from the queue |
| `builder.WithGracefulShutdownTimeout(d)` | `30s` | drain window for in-flight reconciles after SIGTERM |

## Examples

- [examples/quickstart](examples/quickstart) — compilable `main.go`: no-op `ExternalClient` reconciling a GVR via the builder.
- [examples/integration](examples/integration) — full integration test on a local kind cluster (`-tags integration`).

## Docs

- [docs/index.md](docs/index.md) — the map
- [docs/overview.md](docs/overview.md) — the reconcile loop and recovery semantics, traced to source
- [docs/usage.md](docs/usage.md) — `go get` + minimal code to a running controller
- [docs/configuration.md](docs/configuration.md) — the whole config surface (options, env)
- [docs/api.md](docs/api.md) — the exported Go API surface
- [docs/examples.md](docs/examples.md) — examples index
- [docs/release.md](docs/release.md) — how a release ships (tag-only)
- [docs/log.md](docs/log.md) — curated history

## Develop & release

`go build ./... && go test -race ./...` — release runbook: [docs/release.md](docs/release.md).
