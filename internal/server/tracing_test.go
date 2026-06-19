package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func TestConfigureTracingExportsOTLPSpans(t *testing.T) {
	var exports int64
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Fatalf("collector path = %s, want /v1/traces", r.URL.Path)
		}
		atomic.AddInt64(&exports, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdown, err := ConfigureTracing(ctx, Config{OTLPEndpoint: collector.URL})
	if err != nil {
		t.Fatalf("ConfigureTracing: %v", err)
	}
	tracer := otel.Tracer("cashflux-test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown tracing: %v", err)
	}
	if atomic.LoadInt64(&exports) == 0 {
		t.Fatal("collector did not receive an OTLP export")
	}
}
