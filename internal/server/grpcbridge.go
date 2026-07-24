// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// syncTransferLog records each sync RPC that crosses the wire so an operator can confirm transfers
// happen and, for a workspace push, see the payload size and whether it arrived as client-side
// ciphertext (a cryptobox envelope) or as plaintext JSON. It writes to stderr in the same key="value"
// style as the tunnel's own logs.
var syncTransferLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// atRest classifies a pushed dataset: a cryptobox envelope means the server stores ciphertext only
// (zero-knowledge); anything else is readable by the server.
func atRest(dataset []byte) string {
	if len(dataset) == 0 {
		return "none"
	}
	if cryptobox.IsEnvelope(dataset) {
		return "ciphertext(encrypted)"
	}
	return "PLAINTEXT(not-encrypted)"
}

// syncTransferInterceptor logs every sync RPC after it runs: the method, the authenticated user, and
// — for PutWorkspace — the workspace, byte count, and encryption status of the dataset.
func syncTransferInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		attrs := []any{"event", "sync_transfer", "method", info.FullMethod, "dur_ms", time.Since(start).Milliseconds()}
		if u, ok := AuthUserFromContext(ctx); ok {
			attrs = append(attrs, "user", u.ID)
		}
		if pr, ok := req.(backendrpc.PutWorkspaceRequest); ok {
			attrs = append(attrs, "workspace", pr.Workspace.ID, "dataset_bytes", len(pr.Dataset), "at_rest", atRest(pr.Dataset))
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
			syncTransferLog.Error("sync rpc", attrs...)
		} else {
			syncTransferLog.Info("sync rpc", attrs...)
		}
		return resp, err
	}
}

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
	RegisterAuthServiceServer(grpcServer, newAuthService(store, cfg))
	RegisterAccountServiceServer(grpcServer, newAccountService(store, cfg))
	RegisterBillingServiceServer(grpcServer, newBillingService(store, cfg))
	RegisterBlobServiceServer(grpcServer, newBlobService(store, cfg))
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
		grpc.ChainUnaryInterceptor(RequestIDUnaryInterceptor(), AuthUnaryInterceptor(grpcTokenValidator(cfg)), syncTransferInterceptor(), LoggingUnaryInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementUnaryInterceptor(cfg, store)),
		grpc.ChainStreamInterceptor(RequestIDStreamInterceptor(), AuthStreamInterceptor(grpcTokenValidator(cfg)), LoggingStreamInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementStreamInterceptor(cfg, store)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	)
	RegisterSyncServiceServer(grpcServer, NewSyncServiceWithLimits(store, cfg.GRPCMaxStreamsPerUser, cfg.Metrics))
	const customAuthEnabled = false // SyncService only — no AuthServiceServer registered on this bridge
	tunnel := grpctunnel.Wrap(grpcServer,
		grpctunnel.WithOriginCheck(func(r *http.Request) bool { return allowedOrigin(r.Header.Get("Origin"), cfg.AppOrigin) }),
		grpctunnel.WithReadLimitBytes(cfg.GRPCReadLimitBytes),
		grpctunnel.WithKeepalive(cfg.GRPCKeepaliveInterval, cfg.GRPCIdleTimeout),
		grpctunnel.WithMaxActiveConnections(cfg.GRPCMaxActiveConnections),
		grpctunnel.WithMaxConnectionsPerClient(cfg.GRPCMaxConnectionsPerClient),
		grpctunnel.WithMaxUpgradesPerClientPerMinute(cfg.GRPCMaxUpgradesPerClientPerMinute),
	)
	// The sync engine's contract is the /grpc tunnel plus the /v1/version discovery handshake: the
	// frontend GETs /v1/version to confirm the backend is reachable and learn its auth mode before it
	// will connect. Serve exactly those two — no billing/portal/OAuth/blob HTTP surface.
	mux := http.NewServeMux()
	mux.Handle("/grpc", tunnel)
	mux.HandleFunc("OPTIONS /v1/version", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		writeJSON(w, VersionResponse{
			APIVersion:          APIVersion,
			MinClientAPIVersion: MinClientAPIVersion,
			AuthMode:            cfg.AuthMode,
			BillingEnabled:      cfg.Billing,
			AuthProviders:       cfg.OAuthProviderNames(),
			PaymentProviders:    cfg.ConfiguredPaymentProviders(),
			CustomAuthEnabled:   customAuthEnabled,
		})
	})
	return mux
}

// pairingOnlyAuthServer wraps an AuthServiceServer and disables Register
// (open username/password self-signup) permanently: NewSyncAndAuthBridgeHandler's
// embedding is a single-owner, invite-only deployment — every account
// creation must go through admin approval (RequestDevicePairing +
// the admin console's approve action, TODOS.md C454), never open self-signup.
// Login stays enabled: once a device completes the pairing bootstrap and
// calls SetPassword, Login is exactly how every subsequent visit signs back
// in — disabling it would make SetPassword pointless.
type pairingOnlyAuthServer struct {
	authServiceServer
}

func (pairingOnlyAuthServer) Register(context.Context, backendrpc.RegisterRequest) (backendrpc.TokenPairResponse, error) {
	return backendrpc.TokenPairResponse{}, status.Error(codes.Unimplemented, "open self-service enrollment is not available on this server")
}

// NewSyncAndAuthBridgeHandler builds a GoGRPCBridge WebSocket handler exposing
// SyncService + AuthService + BlobService — no AIService, no
// AccountService/BillingService, no HTTP site. It is the embeddable slice for
// a host that wants the full per-person sync engine (admin-approved device
// pairing, device sessions, artifact transfer) with NO billing/tier concept:
// every enrolled account gets full access, gated only by admin approval at
// account creation and by cfg's ordinary storage caps thereafter (see
// pkg/embed.NewSyncAndAuthBridge). AuthService's Register (open
// username/password self-signup) is deliberately disabled — see
// pairingOnlyAuthServer.
//
// The CloudEntitlement interceptors stay in the chain even though this
// deployment has no billing: with cfg.Billing == false, IsCloudActive is a
// no-op past the suspension check (see entitlements.go), so keeping them costs
// nothing and gives the operator a working moderation lever — suspend a
// user's row and their Sync/Blob calls start failing — for free. AuthService's
// pre-auth methods are already exempt from both checks via
// authInterceptorSkipMethods.
func NewSyncAndAuthBridgeHandler(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RequestIDUnaryInterceptor(), AuthUnaryInterceptor(grpcTokenValidator(cfg)), syncTransferInterceptor(), LoggingUnaryInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementUnaryInterceptor(cfg, store)),
		grpc.ChainStreamInterceptor(RequestIDStreamInterceptor(), AuthStreamInterceptor(grpcTokenValidator(cfg)), LoggingStreamInterceptor(cfg.Logger, cfg.Metrics), CloudEntitlementStreamInterceptor(cfg, store)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	)
	RegisterSyncServiceServer(grpcServer, NewSyncServiceWithLimits(store, cfg.GRPCMaxStreamsPerUser, cfg.Metrics))
	RegisterAuthServiceServer(grpcServer, pairingOnlyAuthServer{newAuthService(store, cfg)})
	RegisterBlobServiceServer(grpcServer, newBlobService(store, cfg))
	tunnel := grpctunnel.Wrap(grpcServer,
		grpctunnel.WithOriginCheck(func(r *http.Request) bool { return allowedOrigin(r.Header.Get("Origin"), cfg.AppOrigin) }),
		grpctunnel.WithReadLimitBytes(cfg.GRPCReadLimitBytes),
		grpctunnel.WithKeepalive(cfg.GRPCKeepaliveInterval, cfg.GRPCIdleTimeout),
		grpctunnel.WithMaxActiveConnections(cfg.GRPCMaxActiveConnections),
		grpctunnel.WithMaxConnectionsPerClient(cfg.GRPCMaxConnectionsPerClient),
		grpctunnel.WithMaxUpgradesPerClientPerMinute(cfg.GRPCMaxUpgradesPerClientPerMinute),
	)
	// Same discovery contract as NewSyncBridgeHandler: the /grpc tunnel plus
	// /v1/version so the frontend can confirm reachability and auth mode
	// before connecting.
	mux := http.NewServeMux()
	mux.Handle("/grpc", tunnel)
	mux.HandleFunc("OPTIONS /v1/version", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		writeJSON(w, VersionResponse{
			APIVersion:          APIVersion,
			MinClientAPIVersion: MinClientAPIVersion,
			AuthMode:            cfg.AuthMode,
			BillingEnabled:      cfg.Billing,
			AuthProviders:       cfg.OAuthProviderNames(),
			PaymentProviders:    cfg.ConfiguredPaymentProviders(),
			CustomAuthEnabled:   true,  // AuthServiceServer (pairing-only) is registered on this bridge
			RegistrationOpen:    false, // Register is blocked — see pairingOnlyAuthServer
		})
	})
	return mux
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
	// A signed session-JWT access token is checked regardless of cfg.AuthMode.
	// This used to be gated on AuthMode=="oauth" (third-party OAuth "cloud"
	// sign-in, the only source of these JWTs when that gate was written), but
	// AuthService (TODOS.md C418) mints the exact same JWT shape for "Custom
	// Sync" password/pairing enrollment, whose entire premise is working
	// against a plain self-hosted server with AuthMode=="token" and NO OAuth
	// provider configured — that's what "a fixed, built-in server endpoint"
	// (C419) means. Leaving the oauth-only gate in place made a self-hosted
	// Custom Sync session look "signed in" (Register/Login/RedeemPairingCode/
	// RefreshToken are all interceptor-exempt, see authinterceptor_skip.go)
	// while every OTHER authenticated call it needs — ListDevices,
	// AccountService.GetEntitlement, and the SyncService/BlobService calls
	// that are the actual point of syncing — was silently rejected
	// Unauthenticated, eventually degrading to local-only (C427) with no
	// visible error. This check is purely additive: it only ever matches a
	// token that already failed the static cfg.Token/TokenSHA256 comparison
	// above AND verifies against cfg.SessionKey (falling back to MasterKey,
	// same as every other session-signing call site), so a self-host
	// deployment that never issues any AuthService session has nothing new to
	// accept.
	if userID, ok := verifySessionToken(cfg, token, "access", time.Now().UTC()); ok {
		return AuthUser{ID: userID, Token: token}, true
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
