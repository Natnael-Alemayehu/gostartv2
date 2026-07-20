package logging

import (
	"bytes"
	"context"
	"gostartv2/internal/config"
	"log/slog"
	"testing"
)

func TestNew_ProductionUsesJSON(t *testing.T) {
	cfg := &config.Config{
		IsProd: true,
	}

	var buf bytes.Buffer

	logger := NewHandler(&buf, cfg)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"msg":"test message"`)) {
		t.Errorf("expected JSON log output, got: %s", output)
	}

	if !bytes.Contains(buf.Bytes(), []byte(`"key":"value"`)) {
		t.Errorf("expected key=value in JSON, got: %s", output)
	}
}

func TestNew_DevelopmentUsesText(t *testing.T) {
	cfg := &config.Config{
		IsProd: false,
		IsDev:  true,
	}

	var buf bytes.Buffer

	logger := NewHandler(&buf, cfg)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("test message")) {
		t.Errorf("expected text log output containing message, got: %s", output)
	}
}

func TestFromContext_DefaultWhenAbsent(t *testing.T) {
	logger := FromContext(context.Background())
	if logger == nil {
		t.Fatal("FromContext returned nil")
	}
}

func TestWithLogger_FromContext_Roundtrip(t *testing.T) {
	var buf bytes.Buffer

	customLogger := slog.New(slog.NewTextHandler(&buf, nil))

	ctx := WithLogger(context.Background(), customLogger)
	got := FromContext(ctx)

	if got != customLogger {
		t.Error("FromContext did not return the logger set by WithLogger")
	}
}
