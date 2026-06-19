package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

func TestRequestLogMiddlewareWritesRequestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, Config{LogFormat: "json", LogLevel: "info"})
	h := requestIDMiddleware(requestLogMiddleware(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetLogScope(r.Context(), LogScope{WorkspaceID: "w-http", DeviceID: "d-http"})
		w.WriteHeader(http.StatusAccepted)
	})))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set(requestIDHeader, "http-1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	out := buf.String()
	for _, want := range []string{`"request_id":"http-1"`, `"method":"GET"`, `"route":"/readyz"`, `"status":202`, `"workspace_id":"w-http"`, `"device_id":"d-http"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %s in %s", want, out)
		}
	}
}

func TestLoggingUnaryInterceptorWritesRPCFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, Config{LogFormat: "json", LogLevel: "info"})
	metrics := NewMetrics()
	ctx := ContextWithAuthUser(ContextWithRequestID(context.Background(), "rpc-1"), AuthUser{ID: "u1"})
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-request-id", "rpc-1"))
	interceptor := LoggingUnaryInterceptor(logger, metrics)
	if _, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/cashflux.v1.SyncService/ListWorkspaces"}, func(ctx context.Context, req any) (any, error) {
		SetLogScope(ctx, LogScope{WorkspaceID: "w1", DeviceID: "browser-a"})
		return "ok", nil
	}); err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"request_id":"rpc-1"`, `"rpc":"/cashflux.v1.SyncService/ListWorkspaces"`, `"status":"OK"`, `"user_id":"u1"`, `"workspace_id":"w1"`, `"device_id":"browser-a"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %s in %s", want, out)
		}
	}
	var metricsOut bytes.Buffer
	metrics.WritePrometheus(&metricsOut)
	if !strings.Contains(metricsOut.String(), `cashflux_grpc_requests_total{method="/cashflux.v1.SyncService/ListWorkspaces",status="OK"} 1`) {
		t.Fatalf("metrics missing grpc count: %s", metricsOut.String())
	}
}
