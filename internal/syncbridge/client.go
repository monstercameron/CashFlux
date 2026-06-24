// SPDX-License-Identifier: MIT

// Package syncbridge builds the browser-to-backend gRPC tunnel transport.
package syncbridge

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const defaultGRPCPath = "/grpc"

// Config carries the backend endpoint and bearer token saved in local prefs.
type Config struct {
	ServerURL string
	Token     string
}

// Target converts the configured backend HTTP(S) URL to the websocket /grpc tunnel URL.
func Target(serverURL string) (string, error) {
	raw := strings.TrimSpace(serverURL)
	if raw == "" {
		return "", fmt.Errorf("sync bridge: server url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("sync bridge: parse server url: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("sync bridge: server url host is required")
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("sync bridge: unsupported server url scheme %q", u.Scheme)
	}
	if strings.TrimRight(u.Path, "/") != defaultGRPCPath {
		u.Path = strings.TrimRight(u.Path, "/") + defaultGRPCPath
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// TunnelConfig returns the typed GoGRPCBridge config plus auth interceptors.
func TunnelConfig(cfg Config, extra ...grpc.DialOption) (grpctunnel.TunnelConfig, error) {
	target, err := Target(cfg.ServerURL)
	if err != nil {
		return grpctunnel.TunnelConfig{}, err
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return grpctunnel.TunnelConfig{}, fmt.Errorf("sync bridge: bearer token is required")
	}
	opts := append([]grpc.DialOption{}, extra...)
	opts = grpctunnel.ApplyTunnelInsecureCredentials(opts)
	opts = append(opts,
		grpc.WithUnaryInterceptor(UnaryBearerInterceptor(token)),
		grpc.WithStreamInterceptor(StreamBearerInterceptor(token)),
	)
	return grpctunnel.TunnelConfig{Target: target, GRPCOptions: opts}, nil
}

// Dial opens a gRPC ClientConn through the backend websocket bridge.
func Dial(ctx context.Context, cfg Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	tunnel, err := TunnelConfig(cfg, extra...)
	if err != nil {
		return nil, err
	}
	return grpctunnel.BuildTunnelConn(ctx, tunnel)
}

// UnaryBearerInterceptor attaches Authorization metadata to unary RPCs.
func UnaryBearerInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withBearer(ctx, token), method, req, reply, cc, opts...)
	}
}

// StreamBearerInterceptor attaches Authorization metadata to streaming RPCs.
func StreamBearerInterceptor(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withBearer(ctx, token), desc, cc, method, opts...)
	}
}

func withBearer(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+strings.TrimSpace(token))
}
