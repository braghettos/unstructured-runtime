package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestNewOTelHandler_TeesJSON verifies the OTLP-capable handler still writes the OTel-model JSON
// stream (the otelslog bridge is a no-op with no LoggerProvider installed, so no record is lost).
func TestNewOTelHandler_TeesJSON(t *testing.T) {
	var buf bytes.Buffer
	h := NewOTelHandler(slog.LevelInfo, &buf, "cdc")
	slog.New(h).Info("teed", "a", "b")
	out := buf.String()
	if !strings.Contains(out, `"msg":"teed"`) || !strings.Contains(out, `"SeverityNumber":9`) {
		t.Errorf("tee did not write the OTel-model JSON stream: %s", out)
	}
	if !strings.Contains(out, `"service.name":"cdc"`) {
		t.Errorf("service.name not set by NewOTelHandler: %s", out)
	}
}
