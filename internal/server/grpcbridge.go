// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func NewGRPCBridgeHandler(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RequestIDUnaryInterceptor(), AuthUnaryInterceptor(grpcTokenValidator(cfg)), LoggingUnaryInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementUnaryInterceptor(cfg, store)),
		grpc.ChainStreamInterceptor(RequestIDStreamInterceptor(), AuthStreamInterceptor(grpcTokenValidator(cfg)), LoggingStreamInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementStreamInterceptor(cfg, store)),
		// Permit the client's ~40s keepalive PINGs (syncbridge clientKeepaliveInterval)
		// during an active watch stream so a half-open connection is detected
		// client-side, without earning a GOAWAY. MinTime is set below that interval;
		// pings without an active stream are still not permitted.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	)
	RegisterSyncServiceServer(grpcServer, NewSyncServiceWithLimits(store, cfg.GRPCMaxStreamsPerUser, cfg.Metrics))
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

// NewSyncBridgeHandler builds a GoGRPCBridge WebSocket handler that exposes ONLY the data-sync
// gRPC service (SyncService) — no AIService, no HTTP site. It is the embeddable slice of the
// server for hosts that want just the encrypted workspace-sync engine over gRPC-over-WebSocket,
// reusing the same auth (token), request-ID, logging, entitlement, and keepalive machinery as the
// full bridge so an existing CashFlux frontend syncs against it unchanged.
func NewSyncBridgeHandler(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RequestIDUnaryInterceptor(), AuthUnaryInterceptor(grpcTokenValidator(cfg)), LoggingUnaryInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementUnaryInterceptor(cfg, store)),
		grpc.ChainStreamInterceptor(RequestIDStreamInterceptor(), AuthStreamInterceptor(grpcTokenValidator(cfg)), LoggingStreamInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementStreamInterceptor(cfg, store)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	)
	RegisterSyncServiceServer(grpcServer, NewSyncServiceWithLimits(store, cfg.GRPCMaxStreamsPerUser, cfg.Metrics))
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
	if strings.EqualFold(cfg.AuthMode, "oauth") {
		if userID, ok := verifySessionToken(cfg, token, "access", time.Now().UTC()); ok {
			return AuthUser{ID: userID, Token: token}, true
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

// matchesStaticToken reports whether token is the configured static server token
// (plaintext CASHFLUX_SERVER_TOKEN or its sha256 CASHFLUX_SERVER_TOKEN_SHA256).
// In self-host token mode, possessing this token IS operator authority — so
// operator-only surfaces (audit, metrics) accept it without also requiring the
// token's synthetic id to be listed in AdminUserIDs. Constant-time throughout.
func (c Config) matchesStaticToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if expected := strings.TrimSpace(c.Token); expected != "" &&
		subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
		return true
	}
	if expectedHash := strings.TrimSpace(c.TokenSHA256); expectedHash != "" {
		sum := sha256.Sum256([]byte(token))
		if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(expectedHash)) == 1 {
			return true
		}
	}
	return false
}
