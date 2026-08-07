---
type: Example
title: unstructured-runtime — integration example
description: End-to-end integration test on a local kind cluster — CRD install, controller run, Observe/Create/Delete assertions.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [example, integration, kind, e2e-framework]
timestamp: 2026-08-07T00:00:00Z
---

# Integration example

[sample_test.go](./sample_test.go) (build tag `integration`) is the full
end-to-end path: it creates a **local kind cluster** with
`sigs.k8s.io/e2e-framework`, installs the sample CRD from
[crds/](./crds/finops.krateo.io_focusconfigs.yaml), builds a controller for
`finops.krateo.io/v1 focusconfigs` via the builder (custom logger, naive local
pluralizer, 2s resync), registers a recording no-op `ExternalClient`, then
creates / updates / deletes a CR and asserts that Observe, Create and Delete
were all driven by the runtime. The cluster is destroyed on teardown.

## Preconditions

- Go 1.25+, Docker running, and `kind` in `PATH`.

## Run

```sh
go test -tags integration -v ./examples/integration/
```

The build tag keeps it out of ordinary `go build ./...` / `go test ./...`.
