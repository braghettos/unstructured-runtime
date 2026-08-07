---
type: Configuration
title: unstructured-runtime — configuration
description: The whole config surface a consumer can set — builder options, controller.Options, telemetry.Config — and the environment variables the library reads.
resource: github.com/krateo-platformops/unstructured-runtime
tags: [options, builder, otel, env]
timestamp: 2026-08-07T00:00:00Z
---

# Configuration

A library has no values.yaml: its config surface is the options its consumers
pass in code, plus the few environment variables it reads.

## Builder options (`pkg/controller/builder`)

`builder.Build(ctx, builder.Configuration{…}, opts…)` — required configuration:

| Field | Meaning |
|---|---|
| `Config` | the `*rest.Config` (use `builder.GetConfig()` in-cluster; it disables client-side rate limiting, `QPS = -1`, relying on API Priority & Fairness) |
| `GVR` | the GroupVersionResource to reconcile |
| `ProviderName` | the source name for the Kubernetes event recorders |

Functional options (`pkg/controller/builder/options.go`), all optional:

| Option | Default | Effect |
|---|---|---|
| `WithNamespace(ns)` | `""` (all namespaces) | scope the watch |
| `WithResyncInterval(d)` | `3m` | informer resync → periodic Low-priority Observe of every CR |
| `WithLogger(l)` | no-op logger | a `pkg/logging.Logger` (adapters for `logr` and `slog`) |
| `WithPluralizer(p)` | discovery-backed `pluralizer.New()` (TTL-cached) | GVK→GVR mapping used by CR update/status writes |
| `WithListWatcher(cfg)` | none | label/field selectors for the watch |
| `WithGlobalRateLimiter(rl)` | max of per-item exponential 3s→180s and global 10 qps / burst 100 | retry pacing for failed events |
| `WithMaxRetries(n)` | `5` | failed-event retries before it is dropped (next resync re-picks the object) |
| `WithMetrics(metricsserver.Options)` | `BindAddress: "0"` (disabled) | Prometheus endpoint serving `/metrics`; an unspecified `BindAddress` in `metricsserver.Options` means `:8080` |
| `WithTelemetryMetrics(m)` | nil (off) | OTel metrics handle from `telemetry.Setup` |
| `WithWatchAnnotations(anns…)` | pause→Observe, external-create-pending deletion→Create | annotation-change → event-type triggers |
| `WithActionEvent(action, evType)` | Created→Observe, Updated→Update, Deleted→Delete | remap which queue event a CR lifecycle action produces |
| `WithGracefulShutdownTimeout(d)` | `30s` | drain window after ctx cancel; `0` = abrupt, negative = wait forever; **must be below the pod's `terminationGracePeriodSeconds`** |

Consumers bypassing the builder use `controller.New(sid, controller.Options{…})`
directly — same knobs, plus explicit recorders, dynamic client and metrics
server (`pkg/controller/controller.go:73-99`).

## Telemetry (`pkg/telemetry`, `pkg/logging`)

All export is **off by default** and gated per pipeline:

| Surface | Gate | Notes |
|---|---|---|
| OTel metrics | `telemetry.Setup(ctx, log, telemetry.Config{Enabled: true, …})` | export interval default 30s; resource attrs from `Config` (`ServiceName`, `DeploymentName`, `Namespace`, `Version`, `CompositionID`) |
| OTel traces | `telemetry.SetupTracing(…, Config{TracingEnabled: true})` | W3C propagator installed even when disabled, so inbound `krateo.io/traceparent` is always continued; batch export only |
| OTLP logs | `telemetry.SetupOTLPLogs(ctx, enabled, serviceName)` | must run **before** building the log handler (`logging.NewOTelHandler` tee captures the provider at construction) |

## Environment variables the library reads

| Variable | Read at | Effect |
|---|---|---|
| `DEBUG` | `pkg/context/context.go:34` | `"true"` → the fallback logger built by `context.WithLogger(nil)` logs at debug level (only when the consumer passed no logger) |
| `OTEL_EXPORTER_OTLP_*` | the OTLP/HTTP exporters created in `telemetry.Setup` / `SetupTracing` / `SetupOTLPLogs` | standard OpenTelemetry SDK env (endpoint, headers, protocol) for metrics, traces and logs |

There are no build tags affecting the library itself; the only build tag in
the repo is `integration` on
[examples/integration](../examples/integration/sample_test.go), which gates
the kind-cluster example out of ordinary `go build ./...` / `go test ./...`.
