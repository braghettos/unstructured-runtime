package logging

import (
	"context"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// otelSeverityNumber maps an slog.Level to the OpenTelemetry SeverityNumber.
// (TRACE=1, DEBUG=5, INFO=9, WARN=13, ERROR=17, FATAL=21.)
func otelSeverityNumber(l slog.Level) int {
	switch {
	case l < slog.LevelDebug:
		return 1 // TRACE
	case l < slog.LevelInfo:
		return 5 // DEBUG
	case l < slog.LevelWarn:
		return 9 // INFO
	case l < slog.LevelError:
		return 13 // WARN
	default:
		return 17 // ERROR
	}
}

// NewOTelJSONHandler returns an slog.Handler that emits one JSON object per line
// in the OpenTelemetry log data model:
//   - `timestamp` as RFC3339Nano UTC,
//   - the slog level string (SeverityText) plus a sibling `SeverityNumber`,
//   - the message as `msg` (Body) only — structured fields stay as attributes,
//   - `trace_id`/`span_id` whenever a span is present in the record's context.
//
// Any attrs passed here become resource-level attributes carried on every record
// (e.g. service.name). It is a NEW factory, deliberately independent of
// NewSlogLogger/NewLogrLogger so their logr/slog parity (logging_test.go) is
// unaffected.
func NewOTelJSONHandler(level slog.Leveler, w io.Writer, attrs ...slog.Attr) slog.Handler {
	replace := func(groups []string, a slog.Attr) slog.Attr {
		// Only rewrite top-level keys; never touch nested time/level keys.
		if len(groups) != 0 {
			return a
		}
		if a.Key == slog.TimeKey {
			a.Key = "timestamp"
			a.Value = slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano))
		}
		return a
	}
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		AddSource:   false,
		ReplaceAttr: replace,
	}).WithAttrs(attrs)
	return &otelHandler{Handler: base}
}

// otelHandler enriches each record with SeverityNumber and, when a span is in
// context, trace_id/span_id — without polluting the Body.
type otelHandler struct {
	slog.Handler
}

func (o *otelHandler) Handle(ctx context.Context, rec slog.Record) error {
	rec.AddAttrs(slog.Int("SeverityNumber", otelSeverityNumber(rec.Level)))
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return o.Handler.Handle(ctx, rec)
}

func (o *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &otelHandler{Handler: o.Handler.WithAttrs(attrs)}
}

func (o *otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{Handler: o.Handler.WithGroup(name)}
}
