package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLoggerRedactsSensitiveAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, Config{LogFormat: "json", LogLevel: "debug"})
	logger.Info("saved", "token", "abc123", "api_key", "sk-secret", "route", "/readyz")
	out := buf.String()
	if strings.Contains(out, "abc123") || strings.Contains(out, "sk-secret") {
		t.Fatalf("log leaked secret: %s", out)
	}
	if strings.Count(out, "[REDACTED]") != 2 || !strings.Contains(out, `"/readyz"`) {
		t.Fatalf("unexpected redacted log: %s", out)
	}
}

func TestNewLoggerHonorsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, Config{LogFormat: "text", LogLevel: "warn"})
	logger.Info("skip")
	logger.Warn("keep")
	out := buf.String()
	if strings.Contains(out, "skip") || !strings.Contains(out, "keep") {
		t.Fatalf("level output = %q", out)
	}
}

func TestConfigValidateRejectsBadLogConfig(t *testing.T) {
	cfg := Config{Addr: ":0", DataDir: t.TempDir(), AuthMode: "token", LogFormat: "xml"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad log format accepted")
	}
	cfg.LogFormat = "json"
	cfg.LogLevel = "trace"
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad log level accepted")
	}
}
