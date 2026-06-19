package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDMiddlewarePropagatesHTTPHeader(t *testing.T) {
	var got string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	req.Header.Set(requestIDHeader, "req-test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "req-test" || rr.Header().Get(requestIDHeader) != "req-test" {
		t.Fatalf("request id context/header = %q/%q", got, rr.Header().Get(requestIDHeader))
	}
}

func TestRequestIDMiddlewareGeneratesHTTPHeader(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"})
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("livez status = %d", rr.Code)
	}
	if rr.Header().Get(requestIDHeader) == "" {
		t.Fatal("missing generated request id header")
	}
}

func TestRequestIDUnaryInterceptorPropagatesMetadata(t *testing.T) {
	interceptor := RequestIDUnaryInterceptor()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "rpc-1"))
	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, func(ctx context.Context, req any) (any, error) {
		id, ok := RequestIDFromContext(ctx)
		if !ok || id != "rpc-1" {
			t.Fatalf("request id = %q/%v", id, ok)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
}
