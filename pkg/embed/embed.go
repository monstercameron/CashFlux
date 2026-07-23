// Package embed exposes CashFlux's data-sync engine as an embeddable gRPC-over-WebSocket handler so
// another Go service can host CashFlux's multi-device sync as a managed backend in-process — owning
// the encrypted server-side SQLite store (cashflux-server.db) rather than running a separate
// cashflux-server process.
//
// It deliberately exposes ONLY the sync engine (SyncService over the GoGRPCBridge tunnel), not the
// full CashFlux HTTP site (no billing/portal/console, no AI proxy). The embedding server mounts the
// returned handler at the WebSocket bridge path the CashFlux frontend dials (/grpc) and points the
// frontend's "server URL" at its own origin.
//
// It is a thin public wrapper over internal/server (which the rest of the module reaches only via
// internal imports); external modules import this package.
package embed

import (
	"net/http"
	"path/filepath"

	"github.com/monstercameron/CashFlux/internal/server"
)

// NewSyncBridge opens the encrypted store under dataDir and returns CashFlux's data-sync bridge
// handler (SyncService over gRPC-over-WebSocket), a close function to run at shutdown, and the
// access token to surface when one was auto-generated.
//
// Configuration is read from the environment (CASHFLUX_SERVER_*); a non-empty dataDir overrides
// CASHFLUX_SERVER_DATA_DIR. The returned handler serves only the sync engine — mount it at the
// bridge path the frontend dials (/grpc). Auth follows the configured mode (token by default), so
// the frontend must present its server token to sync.
//
// In token mode with no CASHFLUX_SERVER_TOKEN / CASHFLUX_SERVER_TOKEN_SHA256 set, a fresh random
// token is minted each start. The third return value carries that generated token (empty when the
// token was pinned via env) so the embedding server can log it — otherwise it is unrecoverable and
// the frontend can never authenticate. Set CASHFLUX_SERVER_TOKEN to keep it stable across restarts.
func NewSyncBridge(dataDir string) (http.Handler, func() error, string, error) {
	cfg, err := server.FromEnv()
	if err != nil {
		return nil, nil, "", err
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		return nil, nil, "", err
	}
	return server.NewSyncBridgeHandler(cfg, store), store.Close, cfg.TokenForDisplay(), nil
}

// NewSyncAndAuthBridge is NewSyncBridge's per-person sibling: it wires up
// SyncService + AuthService + BlobService (phone/SMS enrollment, device
// sessions, artifact transfer), with no billing/tier concept — every account
// minted via AuthService gets full access. It's for a host that wants CashFlux
// sync for itself and a small, manually-invited set of people (rather than
// NewSyncBridge's single shared static token, where every caller is
// indistinguishable from any other).
//
// New-account creation is gated by CASHFLUX_SERVER_SETUP_CODE
// (server.Config.SetupCode): if set, a brand-new phone number must present it
// to RequestPhoneVerification/VerifyPhoneCode; a phone number that has already
// completed verification once is never asked for it again on later devices.
// Leave the env var unset for open self-service enrollment.
//
// Same configuration source, return shape, and token-generation contract as
// NewSyncBridge — see its doc comment for CASHFLUX_SERVER_* env vars and the
// generated-token caveat.
func NewSyncAndAuthBridge(dataDir string) (http.Handler, func() error, string, error) {
	cfg, err := server.FromEnv()
	if err != nil {
		return nil, nil, "", err
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		return nil, nil, "", err
	}
	return server.NewSyncAndAuthBridgeHandler(cfg, store), store.Close, cfg.TokenForDisplay(), nil
}
