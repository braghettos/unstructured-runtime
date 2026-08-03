package context

import (
	"context"
	"log/slog"
	"os"

	"github.com/krateo-platformops/plumbing/shortid"
	"github.com/krateo-platformops/unstructured-runtime/pkg/logging"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string
type WithContextFunc func(context.Context) context.Context

var (
	contextKeyLogger  = contextKey("logger")
	contextKeyTraceId = contextKey("traceId")
)

func Logger(ctx context.Context) logging.Logger {
	log, ok := ctx.Value(contextKeyLogger).(logging.Logger)
	if !ok {
		log = logging.NewSlogLogger(slog.New(logging.NewOTelJSONHandler(slog.LevelInfo, os.Stderr)))
	}

	return log
}

func WithLogger(root logging.Logger) WithContextFunc {
	return func(ctx context.Context) context.Context {
		if root == nil {
			logLevel := slog.LevelInfo
			if os.Getenv("DEBUG") == "true" {
				logLevel = slog.LevelDebug
			}
			root = logging.NewSlogLogger(slog.New(logging.NewOTelJSONHandler(logLevel, os.Stderr)))
		}

		return context.WithValue(ctx, contextKeyLogger, root)
	}
}

func WithTraceId(traceId string) WithContextFunc {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, contextKeyTraceId, traceId)
	}
}

func TraceId(ctx context.Context, generate bool) string {
	// Prefer the live OTel span's trace ID (the real 32-hex W3C id) so the "traceId" log
	// field correlates with the distributed trace; fall back to the shortid for the
	// no-otel path.
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}

	traceId, ok := ctx.Value(contextKeyTraceId).(string)
	if ok {
		return traceId
	}

	if generate {
		traceId = shortid.MustGenerate()
	}

	return traceId
}

func BuildContext(ctx context.Context, opts ...WithContextFunc) context.Context {
	for _, fn := range opts {
		if fn == nil {
			continue
		}
		ctx = fn(ctx)
	}

	return ctx
}
