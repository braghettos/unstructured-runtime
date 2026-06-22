package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

func TestNewOTelJSONHandler_Shape(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewOTelJSONHandler(slog.LevelInfo, &buf, slog.String("service.name", "test-svc")))
	log.Warn("hello", "k", "v")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("output is not JSON: %v / %q", err, buf.String())
	}
	ts, ok := m["timestamp"].(string)
	if !ok {
		t.Fatalf("missing top-level timestamp: %v", m)
	}
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Fatalf("timestamp not RFC3339Nano: %s", ts)
	}
	if _, ok := m["time"]; ok {
		t.Fatalf("slog time key should be renamed to timestamp")
	}
	if n, _ := m["SeverityNumber"].(float64); int(n) != 13 { // WARN
		t.Fatalf("SeverityNumber for WARN want 13 got %v", m["SeverityNumber"])
	}
	if m["msg"] != "hello" {
		t.Fatalf("Body must be msg-only; want hello got %v", m["msg"])
	}
	if m["service.name"] != "test-svc" {
		t.Fatalf("resource attr service.name missing: %v", m)
	}
	if _, ok := m["trace_id"]; ok {
		t.Fatalf("trace_id must be absent when no span is in context")
	}
}

func TestNewOTelJSONHandler_TraceBridge(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewOTelJSONHandler(slog.LevelInfo, &buf))

	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	log.InfoContext(ctx, "with span")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if m["trace_id"] != tid.String() {
		t.Fatalf("trace_id want %s got %v", tid, m["trace_id"])
	}
	if m["span_id"] != sid.String() {
		t.Fatalf("span_id want %s got %v", sid, m["span_id"])
	}
}
