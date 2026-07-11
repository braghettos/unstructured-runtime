package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	logglobal "go.opentelemetry.io/otel/log/global"
)

// ServiceNameAttr is the persistent OTel resource attribute pair to pass to NewOTelJSONHandler for a
// given service name (the `service.name` OTel attribute plus the legacy `service` kept during the
// log-scraper transition).
func ServiceNameAttr(serviceName string) []slog.Attr {
	return []slog.Attr{
		slog.String("service.name", serviceName),
		slog.String("service", serviceName),
	}
}

// NewOTelHandler returns the recommended slog.Handler for a service: it tees each record to (a) the
// OTel-model JSON stream on w (stderr by default, for log scrapers) and (b) the global OTel
// LoggerProvider via the otelslog bridge, so records are exported over OTLP once the binary installs
// an exporter (telemetry.SetupOTLPLogs). With no LoggerProvider installed the bridge is a no-op and
// only the JSON stream is written, so this is always safe to use.
//
// This package depends only on the OTel log API + otelslog bridge (not the log SDK): the SDK +
// exporter setup lives in the binary next to its trace/metric setup (see pkg/telemetry).
func NewOTelHandler(level slog.Leveler, w io.Writer, serviceName string) slog.Handler {
	jsonH := NewOTelJSONHandler(level, w, ServiceNameAttr(serviceName)...)
	bridge := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(logglobal.GetLoggerProvider()))
	return &teeHandler{handlers: []slog.Handler{jsonH, bridge}}
}

// teeHandler fans a record out to every sub-handler (JSON stream + OTLP bridge).
type teeHandler struct {
	handlers []slog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, rec slog.Record) error {
	var firstErr error
	for _, h := range t.handlers {
		if !h.Enabled(ctx, rec.Level) {
			continue
		}
		if err := h.Handle(ctx, rec.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &teeHandler{handlers: next}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		next[i] = h.WithGroup(name)
	}
	return &teeHandler{handlers: next}
}
