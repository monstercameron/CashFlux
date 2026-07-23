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
	"time"

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

// Bridge is NewSyncAndAuthBridge's return value: the embeddable handler plus
// an Admin management handle, a shutdown func, and the display token.
type Bridge struct {
	// Handler serves the /grpc WebSocket tunnel and /v1/version discovery —
	// mount it at the path the CashFlux frontend dials.
	Handler http.Handler
	// Admin exposes management operations (listing enrolled clients, minting
	// invite codes) against the same underlying store Handler serves.
	Admin *Admin
	// Close releases the underlying store; call it at shutdown.
	Close func() error
	// Token is the auto-generated access token to surface when
	// CASHFLUX_SERVER_TOKEN/_SHA256 were not pinned via env (empty otherwise).
	Token string
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
// (server.Config.SetupCode): if set, a brand-new phone number must present
// either that fixed value or an admin-minted invite code (Bridge.Admin.
// MintInviteCode) to RequestPhoneVerification/VerifyPhoneCode; a phone number
// that has already completed verification once is never asked for it again
// on later devices. Leave the env var unset for open self-service enrollment.
//
// Same configuration source and token-generation contract as NewSyncBridge —
// see its doc comment for CASHFLUX_SERVER_* env vars and the generated-token
// caveat. Returns *Bridge rather than NewSyncBridge's plain tuple since this
// variant has a fourth thing to return (Admin).
func NewSyncAndAuthBridge(dataDir string) (*Bridge, error) {
	cfg, err := server.FromEnv()
	if err != nil {
		return nil, err
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		return nil, err
	}
	return &Bridge{
		Handler: server.NewSyncAndAuthBridgeHandler(cfg, store),
		Admin:   &Admin{store: store},
		Close:   store.Close,
		Token:   cfg.TokenForDisplay(),
	}, nil
}

// Admin exposes management operations for a NewSyncAndAuthBridge deployment:
// listing enrolled phone/SMS clients and minting/listing invite codes. It
// operates directly on the same in-process store the bridge's SyncService/
// AuthService/BlobService serve — no additional HTTP/gRPC surface on the
// CashFlux side.
type Admin struct {
	store *server.Store
}

// PhoneClient is one enrolled phone/SMS account.
type PhoneClient struct {
	ID              string
	PhoneNumber     string
	CreatedAt       time.Time
	PhoneVerifiedAt time.Time
	Suspended       bool
}

// InviteCode is one minted enrollment invite code. ConsumedAt is the zero
// time when the code is still outstanding (unexpired and unredeemed).
type InviteCode struct {
	Code       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt time.Time
}

// ListClients returns enrolled phone/SMS accounts, newest first.
func (a *Admin) ListClients() ([]PhoneClient, error) {
	rows, err := a.store.ListPhoneClients(0)
	if err != nil {
		return nil, err
	}
	out := make([]PhoneClient, 0, len(rows))
	for _, r := range rows {
		out = append(out, PhoneClient{
			ID: r.ID, PhoneNumber: r.PhoneNumber, CreatedAt: r.CreatedAt,
			PhoneVerifiedAt: r.PhoneVerifiedAt, Suspended: r.Suspended,
		})
	}
	return out, nil
}

// MintInviteCode creates a new short-lived, single-use enrollment invite code
// (server.InviteCodeTTL) not tied to any existing account — hand the
// returned code to one specific invitee.
func (a *Admin) MintInviteCode() (code string, expiresAt time.Time, err error) {
	return a.store.MintInviteCode(time.Now().UTC())
}

// ListInviteCodes returns minted invite codes, newest first.
func (a *Admin) ListInviteCodes() ([]InviteCode, error) {
	rows, err := a.store.ListInviteCodes(0)
	if err != nil {
		return nil, err
	}
	out := make([]InviteCode, 0, len(rows))
	for _, r := range rows {
		out = append(out, InviteCode{Code: r.Code, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, ConsumedAt: r.ConsumedAt})
	}
	return out, nil
}
