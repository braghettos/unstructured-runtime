package telemetry

import (
	"context"

	"github.com/krateoplatformops/unstructured-runtime/pkg/logging"
	"github.com/krateoplatformops/unstructured-runtime/pkg/meta"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/krateoplatformops/unstructured-runtime"

// reconcileObject is the minimal surface StartReconcileSpan needs (satisfied by
// *unstructured.Unstructured) — keeps pkg/telemetry free of a k8s dependency.
type reconcileObject interface {
	GetKind() string
	GetName() string
	GetNamespace() string
	GetAnnotations() map[string]string
}

// SetupTracing configures an OTLP trace pipeline and installs the W3C propagator, sharing the
// metrics Resource (buildResource) so traces/metrics/logs all carry identical resource
// attributes. It is a no-op (returning a working no-op shutdown) when cfg.TracingEnabled is
// false, so callers can wire it unconditionally and otel.Tracer(...).Start stays safe.
//
// Uses WithBatcher (NEVER WithSyncer): a slow or dead collector must never block a reconcile.
func SetupTracing(ctx context.Context, log logging.Logger, cfg Config) (func(context.Context) error, error) {
	if log == nil {
		log = logging.NewNopLogger()
	}

	// Install the propagator even when not exporting, so an inbound traceparent is honored
	// and can be continued onto child resources regardless of this process's export config.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if !cfg.TracingEnabled {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	res, err := buildResource(cfg)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	log.Info("OpenTelemetry tracing initialized", "serviceName", cfg.ServiceName, "compositionID", cfg.CompositionID)
	return tp.Shutdown, nil
}

// StartReconcileSpan starts the per-reconcile root span, continuing any cross-composition
// trace carried on the object's krateo.io/traceparent annotation (translated to/from the W3C
// carrier key). Safe to call when tracing is disabled — the global tracer is a no-op.
func StartReconcileSpan(ctx context.Context, obj reconcileObject) (context.Context, trace.Span) {
	if ann := obj.GetAnnotations(); len(ann) > 0 {
		if tp := ann[meta.AnnotationKeyTraceparent]; tp != "" {
			carrier := propagation.MapCarrier{"traceparent": tp}
			if ts := ann[meta.AnnotationKeyTracestate]; ts != "" {
				carrier["tracestate"] = ts
			}
			ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		}
	}
	return otel.Tracer(tracerName).Start(ctx, "reconcile",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("k8s.object.kind", obj.GetKind()),
			attribute.String("k8s.object.name", obj.GetName()),
			attribute.String("k8s.object.namespace", obj.GetNamespace()),
		),
	)
}

// RecordSpanError marks the span failed when err != nil; no-op otherwise.
func RecordSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// InjectTraceparent stores the active W3C trace context on the annotations map under
// krateo.io/traceparent (+ krateo.io/tracestate), so a child resource carries it for the next
// controller to continue the distributed trace. No-op when no span context is active.
func InjectTraceparent(ctx context.Context, annotations map[string]string) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if tp := carrier.Get("traceparent"); tp != "" {
		annotations[meta.AnnotationKeyTraceparent] = tp
		if ts := carrier.Get("tracestate"); ts != "" {
			annotations[meta.AnnotationKeyTracestate] = ts
		}
	}
}
