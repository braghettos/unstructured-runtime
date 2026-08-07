---
type: API
title: unstructured-runtime — API
description: The exported Go API surface, package by package, with the load-bearing types.
resource: https://pkg.go.dev/github.com/krateo-platformops/unstructured-runtime
tags: [go-api, packages]
timestamp: 2026-08-07T00:00:00Z
---

# API

The contract this library exposes is its exported Go surface. Authoritative
reference:
[pkg.go.dev/github.com/krateo-platformops/unstructured-runtime](https://pkg.go.dev/github.com/krateo-platformops/unstructured-runtime).
The load-bearing packages:

## Core

- **`pkg/controller`** — the runtime.
  - `ExternalClient` — the four-method contract you implement:
    ```go
    type ExternalClient interface {
        Observe(ctx context.Context, mg *unstructured.Unstructured) (ExternalObservation, error)
        Create(ctx context.Context, mg *unstructured.Unstructured) error
        Update(ctx context.Context, mg *unstructured.Unstructured) error
        Delete(ctx context.Context, mg *unstructured.Unstructured) error
    }
    ```
  - `ExternalObservation{ResourceExists, ResourceUpToDate bool}`
  - `Options`, `New(sid, Options) (*Controller, error)`
  - `(*Controller).SetExternalClient`, `Run(ctx, numWorkers)`,
    `SetGracefulShutdownTimeout(d)`
  - priorities `LowPriority` (-100) / `NormalPriority` (0) / `HighPriority` (100)
- **`pkg/controller/builder`** — `Configuration`, `Build(ctx, conf, opts…)`,
  `GetConfig()` and the `With…` functional options
  (see [configuration](./configuration.md)).
- **`pkg/controller/event`** — `EventType` (`Observe|Create|Update|Delete`),
  `Event`, `DigestForEvent`, `AnnotationEvent(s)` (trigger rules
  `OnCreate|OnDelete|OnChange|OnAny`), `ActionsEvent` (CR action → event-type
  mapping).
- **`pkg/controller/objectref`** — `ObjectRef{APIVersion, Kind, Name, Namespace}`.
- **`pkg/controller/priorityqueue`** — the generic rate-limited priority queue
  (`PriorityQueue[T]`, `AddOpts{After, RateLimited, Priority}`).

## Managed-resource helpers

- **`pkg/meta`** — the `krateo.io/*` annotation vocabulary and helpers:
  external-name, the `external-create-pending/-succeeded/-failed` handshake
  (`ExternalCreateIncomplete`, `ExternalCreatePendingDuring`, setters/getters),
  `IsPaused`, deletion policy (`ShouldDelete`/orphan), management policy
  (`IsActionAllowed`), traceparent keys, finalizer/label/annotation utilities.
- **`pkg/tools`** — `Update` / `UpdateStatus` of unstructured CRs through the
  dynamic client (GVK→GVR via a `Pluralizer`).
- **`pkg/tools/unstructured`** — condition plumbing on unstructured objects:
  `SetConditions`, `GetCondition(s)`, `IsAvailable`, failed-object-ref helpers.
- **`pkg/tools/unstructured/condition`** — the condition constructors:
  `Available`, `Unavailable`, `Creating`, `Deleting`, `ReconcileSuccess`,
  `ReconcileError(err)`, `ReconcilePaused`, `FailWithReason`.
- **`pkg/tools/statusprojection`** — the jq-based status projection engine:
  `Mapping{For, Expression}` (JSON-tagged wire format), `Project(ctx, cr,
  resolved, mappings)`, `SetObservedGeneration`.
- **`pkg/pluralizer`** — `PluralizerInterface` (`GVKtoGVR`) and the
  discovery-backed, TTL-cached default `New()`.

## Infrastructure

- **`pkg/logging`** — the `Logger` interface (Info/Debug/Warn/Error,
  WithName/WithValues) with `NewLogrLogger`, `NewSlogLogger`, `NewNopLogger`;
  OTel structured-JSON slog handlers `NewOTelJSONHandler` and the OTLP tee
  `NewOTelHandler`.
- **`pkg/telemetry`** — `Config`, `Setup` (OTel metrics; returns the `Metrics`
  handle the controller records into), `SetupTracing`, `SetupOTLPLogs`,
  `StartReconcileSpan`, `InjectTraceparent`.
- **`pkg/metrics`** + **`pkg/metrics/server`** — Prometheus registry,
  workqueue/client-go/leader-election adapters, and the optional metrics
  `Server` (`NewServer(Options, *rest.Config, *http.Client)`).
- **`pkg/listwatcher`** — `Create(CreateOption)` → `*cache.ListWatch` over a
  dynamic client with label/field selectors.
- **`pkg/workqueue`** — rate-limiter helpers and workqueue metrics provider.
- **`pkg/context`** — context plumbing: `BuildContext`, `WithLogger`,
  `WithTraceId`, `Logger(ctx)`, `TraceId(ctx, generate)`.
- **`pkg/signals`** — `SetupSignalHandler()` (SIGTERM/SIGINT stop channel;
  second signal exits 1).
- **`pkg/certwatcher`** — TLS certificate hot-reload from disk for servers.
- **`pkg/errors`** — the wrap/cause error helpers used across Krateo
  controllers (`New`, `Wrap(f)`, `WithMessage(f)`, `Cause`, `Is`, `As`).

`pkg/internal/**` is not part of the public surface.

## Compatibility

The module follows Go semver on `vX.Y.Z` tags (currently `v1.x`): no breaking
changes to the exported surface within the major version. See
[release](./release.md).
