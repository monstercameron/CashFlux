package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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
	RegisterSyncServiceServer(grpcServer, NewSyncService(store))
	RegisterAIServiceServer(grpcServer, newAIService(store, cfg))
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
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthUser{}, false
	}
	expected := strings.TrimSpace(cfg.Token)
	if expected != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
		return authUserFromToken(token), true
	}
	expectedHash := strings.TrimSpace(cfg.TokenSHA256)
	if expectedHash != "" {
		sum := sha256.Sum256([]byte(token))
		got := hex.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(got), []byte(expectedHash)) == 1 {
			return authUserFromToken(token), true
		}
	}
	return AuthUser{}, false
}

func (c Config) TokenForDisplay() string {
	if c.GeneratedToken {
		return c.Token
	}
	return ""
}
