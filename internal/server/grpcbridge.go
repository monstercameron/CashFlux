package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
)

func NewGRPCBridgeHandler(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(AuthUnaryInterceptor(grpcTokenValidator(cfg))),
		grpc.StreamInterceptor(AuthStreamInterceptor(grpcTokenValidator(cfg))),
	)
	RegisterAIServiceServer(grpcServer, newHTTPAIService(store, cfg))
	return grpctunnel.Wrap(grpcServer,
		grpctunnel.WithOriginCheck(func(r *http.Request) bool { return allowedOrigin(r.Header.Get("Origin"), cfg.AppOrigin) }),
		grpctunnel.WithReadLimitBytes(cfg.GRPCReadLimitBytes),
		grpctunnel.WithKeepalive(cfg.GRPCKeepaliveInterval, cfg.GRPCIdleTimeout),
		grpctunnel.WithMaxActiveConnections(cfg.GRPCMaxActiveConnections),
		grpctunnel.WithMaxConnectionsPerClient(cfg.GRPCMaxConnectionsPerClient),
		grpctunnel.WithMaxUpgradesPerClientPerMinute(cfg.GRPCMaxUpgradesPerClientPerMinute),
	)
}

func grpcTokenValidator(cfg Config) TokenValidator {
	return func(_ context.Context, token string) (AuthUser, error) {
		user, ok := authUserForToken(strings.TrimSpace(token), cfg)
		if !ok {
			return AuthUser{}, http.ErrNoCookie
		}
		return user, nil
	}
}

func authUserForToken(token string, cfg Config) (AuthUser, bool) {
	expected := strings.TrimSpace(cfg.Token)
	if token == "" || expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return AuthUser{}, false
	}
	return authUserFromToken(token), true
}
