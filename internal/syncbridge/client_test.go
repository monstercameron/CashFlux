// SPDX-License-Identifier: MIT

package syncbridge

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestTarget(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http://127.0.0.1:8081", "ws://127.0.0.1:8081/grpc"},
		{"https://cashflux.example", "wss://cashflux.example/grpc"},
		{"ws://localhost:8081/grpc", "ws://localhost:8081/grpc"},
		{"wss://api.example/base/", "wss://api.example/base/grpc"},
		{" http://localhost:8081?token=nope#frag ", "ws://localhost:8081/grpc"},
	}
	for _, tt := range tests {
		got, err := Target(tt.in)
		if err != nil {
			t.Fatalf("Target(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("Target(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTargetRejectsInvalidURLs(t *testing.T) {
	for _, in := range []string{"", "http://", "ftp://example.com"} {
		if _, err := Target(in); err == nil {
			t.Fatalf("Target(%q) accepted invalid url", in)
		}
	}
}

func TestTunnelConfigRequiresToken(t *testing.T) {
	_, err := TunnelConfig(Config{ServerURL: "http://127.0.0.1:8081"})
	if err == nil || !strings.Contains(err.Error(), "bearer token") {
		t.Fatalf("TunnelConfig missing token err = %v", err)
	}
	cfg, err := TunnelConfig(Config{ServerURL: "http://127.0.0.1:8081", Token: "dev-token"})
	if err != nil {
		t.Fatalf("TunnelConfig valid: %v", err)
	}
	if cfg.Target != "ws://127.0.0.1:8081/grpc" || len(cfg.GRPCOptions) == 0 {
		t.Fatalf("TunnelConfig = %+v", cfg)
	}
}

func TestUnaryBearerInterceptorAddsMetadata(t *testing.T) {
	interceptor := UnaryBearerInterceptor(" dev-token ")
	err := interceptor(context.Background(), "/cashflux.Sync/List", nil, nil, nil, func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("missing outgoing metadata")
		}
		if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer dev-token" {
			t.Fatalf("authorization metadata = %v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unary interceptor: %v", err)
	}
}

func TestStreamBearerInterceptorAddsMetadata(t *testing.T) {
	interceptor := StreamBearerInterceptor("dev-token")
	_, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/cashflux.Sync/Watch", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("missing outgoing metadata")
		}
		if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer dev-token" {
			t.Fatalf("authorization metadata = %v", got)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("stream interceptor: %v", err)
	}
}
