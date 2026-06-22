package telemetry

import (
	"context"
	"testing"

	"github.com/krateoplatformops/unstructured-runtime/pkg/meta"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// fakeObj implements reconcileObject for tests.
type fakeObj struct {
	kind, name, ns string
	ann            map[string]string
}

func (f fakeObj) GetKind() string                   { return f.kind }
func (f fakeObj) GetName() string                   { return f.name }
func (f fakeObj) GetNamespace() string              { return f.ns }
func (f fakeObj) GetAnnotations() map[string]string { return f.ann }

func TestSetupTracingDisabledIsNoop(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), nil, Config{TracingEnabled: false})
	if err != nil {
		t.Fatalf("SetupTracing(disabled): %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown: %v", err)
	}
	// Must be safe to start/end a span with tracing disabled.
	_, span := StartReconcileSpan(context.Background(), fakeObj{kind: "X", name: "a", ns: "ns"})
	RecordSpanError(span, nil)
	span.End()
}

func TestInjectExtractRoundTrip(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	defer span.End()
	parentTID := span.SpanContext().TraceID().String()

	// Inject the active trace context into a child object's annotations.
	ann := map[string]string{}
	InjectTraceparent(ctx, ann)
	if ann[meta.AnnotationKeyTraceparent] == "" {
		t.Fatalf("InjectTraceparent did not set %s: %v", meta.AnnotationKeyTraceparent, ann)
	}

	// Reconciling that child continues the SAME trace (cross-composition continuity).
	childCtx, childSpan := StartReconcileSpan(context.Background(), fakeObj{kind: "Child", name: "c", ns: "ns", ann: ann})
	defer childSpan.End()
	if got := trace.SpanContextFromContext(childCtx).TraceID().String(); got != parentTID {
		t.Fatalf("child trace ID = %s, want parent %s — cross-composition continuity broken", got, parentTID)
	}
}
