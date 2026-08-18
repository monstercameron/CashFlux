// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterAuthServiceServer registers the hand-rolled AuthService ServiceDesc
// against s. The transport is the same hand-written JSON codec over the
// GoGRPCBridge tunnel that SyncService/AIService already use (see
// sync_grpc.go/ai_grpc.go) — a real protobuf wire format is a later,
// deliberately separate step (TODOS.md C428).
func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv authServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.AuthService",
		HandlerType: (*authServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Enroll", Handler: authEnrollHandler},
			{MethodName: "RedeemPairingCode", Handler: authRedeemPairingCodeHandler},
			{MethodName: "Register", Handler: authRegisterHandler},
			{MethodName: "Login", Handler: authLoginHandler},
			{MethodName: "ResetPassword", Handler: authResetPasswordHandler},
			{MethodName: "RefreshToken", Handler: authRefreshTokenHandler},
			{MethodName: "Logout", Handler: authLogoutHandler},
			{MethodName: "ListDevices", Handler: authListDevicesHandler},
			{MethodName: "RevokeDevice", Handler: authRevokeDeviceHandler},
			{MethodName: "RequestDevicePairing", Handler: authRequestDevicePairingHandler},
			{MethodName: "CancelDevicePairing", Handler: authCancelDevicePairingHandler},
			{MethodName: "SetPassword", Handler: authSetPasswordHandler},
		},
		Streams: []grpc.StreamDesc{
			{StreamName: "WatchPairingStatus", Handler: authWatchPairingStatusHandler, ServerStreams: true},
		},
		Metadata: "cashflux/v1/cashflux.proto",
	}, srv)
}

// AuthServiceServer is the server-side contract for AuthService — the shared
// device/session identity core every "Custom Sync" enrollment door writes into
// (TODOS.md C418). Method shapes mirror the proto 1:1, wrapped in the same
// backendrpc value-type pattern SyncServiceServer/AIServiceServer already use.
type AuthServiceServer interface {
	Enroll(context.Context, backendrpc.EnrollRequest) (backendrpc.TokenPairResponse, error)
	RedeemPairingCode(context.Context, backendrpc.RedeemPairingCodeRequest) (backendrpc.TokenPairResponse, error)
	Register(context.Context, backendrpc.RegisterRequest) (backendrpc.TokenPairResponse, error)
	Login(context.Context, backendrpc.LoginRequest) (backendrpc.TokenPairResponse, error)
	ResetPassword(context.Context, backendrpc.ResetPasswordRequest) (backendrpc.TokenPairResponse, error)
	RefreshToken(context.Context, backendrpc.RefreshTokenRequest) (backendrpc.TokenPairResponse, error)
	Logout(context.Context, backendrpc.LogoutRequest) (backendrpc.LogoutResponse, error)
	ListDevices(context.Context, backendrpc.ListDevicesRequest) (backendrpc.ListDevicesResponse, error)
	RevokeDevice(context.Context, backendrpc.RevokeDeviceRequest) (backendrpc.RevokeDeviceResponse, error)
	RequestDevicePairing(context.Context, backendrpc.RequestDevicePairingRequest) (backendrpc.RequestDevicePairingResponse, error)
	CancelDevicePairing(context.Context, backendrpc.CancelDevicePairingRequest) (backendrpc.CancelDevicePairingResponse, error)
	SetPassword(context.Context, backendrpc.SetPasswordRequest) (backendrpc.SetPasswordResponse, error)
}

// authServiceServer adds the one streaming AuthService method
// (WatchPairingStatus) beyond the unary-only AuthServiceServer — the same
// split syncServiceServer uses beyond SyncServiceServer (see sync_grpc.go),
// needed because grpc.ServiceDesc.Streams handlers take a raw grpc.ServerStream,
// not the (context.Context, request)-shaped unary signature.
type authServiceServer interface {
	AuthServiceServer
	WatchPairingStatusRPC(backendrpc.WatchPairingStatusRequest, grpc.ServerStream) error
}

// authServer implements AuthServiceServer. cfg is required (not just store)
// because RefreshToken/Logout sign and verify session JWTs exactly like the
// existing OAuth HTTP handlers (issueStoredSessionPair/verifySessionClaims,
// both cfg-keyed) — see internal/server/session.go and oauth_http.go.
type authServer struct {
	store *Store
	cfg   Config

	// loginLimiter/registerLimiter/pairingLimiter guard the remaining
	// unauthenticated AuthService doors against brute force: a password and a
	// six-digit pairing code are small-enough guess spaces that an unrated
	// endpoint is a real attack surface (account takeover / device hijack),
	// not just a cost concern. Each is keyed by the best caller-identifying
	// signal available at this layer — the gRPC tunnel carries no client IP
	// (see BearerTokenFromContext/metadata.FromIncomingContext — only
	// "authorization" and "x-request-id" are set), so DeviceLabel/username is
	// the best available signal.
	loginLimiter    *fixedWindowLimiter
	recoveryLimiter *fixedWindowLimiter
	registerLimiter *fixedWindowLimiter
	pairingLimiter  *fixedWindowLimiter

	// pairingGlobalLimiter backstops pairingLimiter with a single,
	// server-wide bucket keyed on a fixed constant (see pairingGlobalLimiterKey)
	// rather than on caller input. pairingLimiter alone is keyed by
	// DeviceLabel, which is an arbitrary, unverified string the caller fully
	// controls — an attacker guessing a pairing code (a 6-digit,
	// PairingCodeTTL-lived, account-takeover secret) can simply send a fresh
	// random label on every request and never trip it. This limiter cannot be
	// evaded that way, since its key never varies with anything the caller
	// sends; it trades a small amount of availability (all callers
	// server-wide share one guess budget) for closing that bypass. The ideal
	// long-term fix is rate-limiting on the caller's real network address the
	// way the HTTP endpoints already do (see rateLimitClientIP in http.go),
	// but the gRPC tunnel doesn't thread that into this layer yet (see this
	// struct's doc comment) — that is a larger, separate change.
	pairingGlobalLimiter *fixedWindowLimiter

	// registerGlobalLimiter is registerGlobalLimiter's pairing-code twin:
	// registerLimiter alone is keyed by the same caller-controlled
	// DeviceLabel, so an attacker rotating it on every call gets unlimited
	// Register attempts — and Register does two bcrypt.DefaultCost hashes per
	// call (password + one-time recovery code), making this a real CPU-
	// exhaustion / account-spam surface, not just a cost concern. See
	// pairingGlobalLimiter's doc comment above for the full rationale; the
	// same trade-off (one server-wide guess budget) applies here.
	registerGlobalLimiter *fixedWindowLimiter
	recoveryGlobalLimiter *fixedWindowLimiter

	// devicePairingLimiter/devicePairingGlobalLimiter guard
	// RequestDevicePairing the same shape as pairingLimiter/
	// pairingGlobalLimiter guard RedeemPairingCode, but against a different
	// threat: this door doesn't let a caller guess an existing secret, it
	// lets them MINT a pending_devices row for free — an attacker rotating
	// DeviceLabel could otherwise spam unbounded rows into the table
	// (storage exhaustion, and a flooded admin approve/reject queue burying
	// the real request). The global backstop closes that label-rotation
	// bypass exactly like pairingGlobalLimiter's doc comment explains.
	devicePairingLimiter       *fixedWindowLimiter
	devicePairingGlobalLimiter *fixedWindowLimiter

	// watchPairingStatusLimiter/watchPairingStatusGlobalLimiter guard
	// WatchPairingStatus (security review finding, TODOS.md C454 follow-up):
	// unlike RequestDevicePairing, this door was originally left completely
	// unrated — an unauthenticated caller could open unbounded concurrent
	// watches (each its own goroutine + 1s poll ticker) on a real or even
	// nonexistent deviceID, degrading the single-connection SQLite store
	// (see OpenStore's SetMaxOpenConns(1)) for every other caller. Keyed by
	// deviceID (a legitimate client opens exactly one watch per request) with
	// the usual constant-keyed global backstop against a caller who fabricates
	// or rotates device ids to evade the per-key limit.
	watchPairingStatusLimiter       *fixedWindowLimiter
	watchPairingStatusGlobalLimiter *fixedWindowLimiter

	// cancelDevicePairingLimiter/cancelDevicePairingGlobalLimiter are
	// CancelDevicePairing's twin — same finding, same shape, same rationale.
	cancelDevicePairingLimiter       *fixedWindowLimiter
	cancelDevicePairingGlobalLimiter *fixedWindowLimiter
}

// pairingGlobalLimiterKey is the single, constant bucket key
// pairingGlobalLimiter uses — deliberately not derived from anything in the
// request, so it cannot be reset by varying request fields.
const pairingGlobalLimiterKey = "redeem-pairing-code:global"

// registerGlobalLimiterKey is registerGlobalLimiter's constant bucket key —
// see pairingGlobalLimiterKey.
const registerGlobalLimiterKey = "register:global"

const recoveryGlobalLimiterKey = "reset-password:global"

// devicePairingGlobalLimiterKey is devicePairingGlobalLimiter's constant
// bucket key — see pairingGlobalLimiterKey.
const devicePairingGlobalLimiterKey = "request-device-pairing:global"

// watchPairingStatusGlobalLimiterKey/cancelDevicePairingGlobalLimiterKey are
// their respective global backstops' constant bucket keys — see
// pairingGlobalLimiterKey.
const (
	watchPairingStatusGlobalLimiterKey  = "watch-pairing-status:global"
	cancelDevicePairingGlobalLimiterKey = "cancel-device-pairing:global"
)

// loginLimitPerMinute/registerLimitPerMinute/pairingLimitPerMinute cap
// attempts against the guessable-secret doors (password, pairing code) per
// username/device.
const (
	loginLimitPerMinute    = 10
	recoveryLimitPerMinute = 5
	registerLimitPerMinute = 5
	pairingLimitPerMinute  = 10

	// pairingGlobalLimitPerMinute caps total RedeemPairingCode attempts across
	// EVERY caller server-wide (see authServer.pairingGlobalLimiter's doc
	// comment on why a per-device cap alone is not enough). Combined with
	// PairingCodeTTL (5 minutes), this bounds a guesser to at most
	// 5*pairingGlobalLimitPerMinute attempts against the full
	// pairingCodeDigits-digit (1-in-a-million) space no matter how many
	// distinct device labels they rotate through — a low success probability
	// per outstanding code, while still being generous for legitimate
	// concurrent device-pairing traffic (a rare, occasional flow, not a
	// high-frequency one like login).
	pairingGlobalLimitPerMinute = 30

	// registerGlobalLimitPerMinute caps total Register attempts across EVERY
	// caller server-wide (see authServer.registerGlobalLimiter's doc comment).
	// Set well above any plausible legitimate concurrent-signup burst but low
	// enough that spamming accounts / burning bcrypt CPU costs the attacker
	// time, not the deployment unbounded CPU no matter how many device labels
	// they rotate through.
	registerGlobalLimitPerMinute = 30
	recoveryGlobalLimitPerMinute = 30

	// devicePairingLimitPerMinute/devicePairingGlobalLimitPerMinute cap
	// RequestDevicePairing the same shape as pairingLimitPerMinute/
	// pairingGlobalLimitPerMinute cap RedeemPairingCode — see
	// authServer.devicePairingLimiter's doc comment.
	devicePairingLimitPerMinute       = 10
	devicePairingGlobalLimitPerMinute = 30

	// watchPairingStatusLimitPerMinute/watchPairingStatusGlobalLimitPerMinute
	// cap WatchPairingStatus — see authServer.watchPairingStatusLimiter's doc
	// comment. Set higher than the mint-side limiters: a legitimate client can
	// reasonably reconnect a few times if a connection drops while waiting on
	// a human admin, which the mint-side doors never need to do.
	watchPairingStatusLimitPerMinute       = 20
	watchPairingStatusGlobalLimitPerMinute = 60

	// cancelDevicePairingLimitPerMinute/cancelDevicePairingGlobalLimitPerMinute
	// cap CancelDevicePairing — see authServer.cancelDevicePairingLimiter's
	// doc comment. A legitimate client cancels at most once per request.
	cancelDevicePairingLimitPerMinute       = 5
	cancelDevicePairingGlobalLimitPerMinute = 30
)

func newAuthService(store *Store, cfg Config) *authServer {
	return &authServer{
		store:                            store,
		cfg:                              cfg,
		loginLimiter:                     newFixedWindowLimiter(loginLimitPerMinute),
		recoveryLimiter:                  newFixedWindowLimiter(recoveryLimitPerMinute),
		registerLimiter:                  newFixedWindowLimiter(registerLimitPerMinute),
		pairingLimiter:                   newFixedWindowLimiter(pairingLimitPerMinute),
		pairingGlobalLimiter:             newFixedWindowLimiter(pairingGlobalLimitPerMinute),
		registerGlobalLimiter:            newFixedWindowLimiter(registerGlobalLimitPerMinute),
		recoveryGlobalLimiter:            newFixedWindowLimiter(recoveryGlobalLimitPerMinute),
		devicePairingLimiter:             newFixedWindowLimiter(devicePairingLimitPerMinute),
		devicePairingGlobalLimiter:       newFixedWindowLimiter(devicePairingGlobalLimitPerMinute),
		watchPairingStatusLimiter:        newFixedWindowLimiter(watchPairingStatusLimitPerMinute),
		watchPairingStatusGlobalLimiter:  newFixedWindowLimiter(watchPairingStatusGlobalLimitPerMinute),
		cancelDevicePairingLimiter:       newFixedWindowLimiter(cancelDevicePairingLimitPerMinute),
		cancelDevicePairingGlobalLimiter: newFixedWindowLimiter(cancelDevicePairingGlobalLimitPerMinute),
	}
}

// Enroll starts a brand-new device/account pairing (the generic entry point
// TODOS.md C419 falls through to when a device has never enrolled before).
// TODO(laneB): see TODOS.md C419 — concrete enrollment doors are Register/
// Login (C422) and RedeemPairingCode (C421).
func (s *authServer) Enroll(context.Context, backendrpc.EnrollRequest) (backendrpc.TokenPairResponse, error) {
	return backendrpc.TokenPairResponse{}, status.Errorf(codes.Unimplemented, "TODO(laneB): see TODOS.md C419")
}

// RedeemPairingCode links a new device to an existing account via a
// portal-minted code (TODOS.md C421). It only ever resolves an EXISTING
// account — a missing, already-consumed, or expired code all fail the same
// way (no account is ever created here, deliberately: see TODOS.md C421).
func (s *authServer) RedeemPairingCode(ctx context.Context, req backendrpc.RedeemPairingCodeRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	code := strings.TrimSpace(req.PairingCode)
	if code == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "pairing code is required")
	}
	now := time.Now().UTC()
	deviceKey := strings.TrimSpace(req.DeviceLabel)
	if deviceKey == "" {
		deviceKey = "unlabeled-device"
	}
	if !s.pairingLimiter.allow(deviceKey, now) || !s.pairingGlobalLimiter.allow(pairingGlobalLimiterKey, now) {
		return backendrpc.TokenPairResponse{}, status.Error(codes.ResourceExhausted, "too many pairing attempts — try again in a minute")
	}
	// A client retry of RedeemPairingCode after a timeout — where it can't
	// know whether the first attempt actually landed — must not fail with
	// "already used" for a code IT successfully consumed a moment ago
	// (TODOS.md C443). PeekPairingCodeUserID resolves the code back to its
	// user id even after consumption (ConsumePairingCode alone cannot: it
	// deliberately fails on a second consume), so the cached token pair can
	// be replayed instead.
	route := backendrpc.MethodAuthRedeemPairingCode
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		if peekedUserID, found, err := s.store.PeekPairingCodeUserID(code); err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "pairing code lookup failed")
		} else if found {
			requestHash := billingRequestHash(route, code, req.DeviceLabel)
			cached, cacheFound, err := s.store.GetIdempotencyResult(peekedUserID, route, idempotencyKey)
			if err != nil {
				return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency lookup failed")
			}
			if cacheFound {
				if cached.RequestHash != requestHash {
					return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "idempotency key was used for a different request")
				}
				var replay backendrpc.TokenPairResponse
				if err := json.Unmarshal(cached.ResponseBody, &replay); err != nil {
					return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency replay decode failed")
				}
				return replay, nil
			}
		}
	}
	userID, ok, err := s.store.ConsumePairingCode(code, now)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "pairing code lookup failed")
	}
	if !ok {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "pairing code is invalid, expired, or already used")
	}
	// C700: close the loop on the request this code came from. An approval
	// nobody redeemed and an approval in daily use are different situations, and
	// the console could not tell them apart. Best-effort: a bookkeeping failure
	// must never cost the user the session they just legitimately earned.
	if _, err := s.store.MarkPendingDeviceRedeemedByCode(code, now); err != nil {
		s.logRedeemBookkeeping(err)
	}
	// The account the code was minted for may have been deleted since minting
	// (e.g. right-to-erasure); IsUserSuspended alone would treat that as "not
	// suspended" and happily issue a session for a user id nothing owns, so
	// existence is checked explicitly first.
	if _, found, err := s.store.GetUserByID(userID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account lookup failed")
	} else if !found {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "pairing code is invalid, expired, or already used")
	}
	if suspended, err := s.store.IsUserSuspended(userID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "suspension check failed")
	} else if suspended {
		return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "account is suspended")
	}
	access, refresh, familyID, err := s.issueSession(userID, now, req.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, userID, "auth.pairing.redeem", "user", userID)
	out := backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
	}
	if idempotencyKey != "" {
		body, err := json.Marshal(out)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "encode pairing response failed")
		}
		if err := s.store.PutIdempotencyResult(IdempotencyResult{
			UserID:       userID,
			Route:        route,
			Key:          idempotencyKey,
			RequestHash:  billingRequestHash(route, code, req.DeviceLabel),
			ResponseBody: body,
			CreatedAt:    now,
		}); err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency store failed")
		}
	}
	return out, nil
}

// Register creates a username/password account for users who won't share a
// phone number (TODOS.md C422). The password is bcrypt-hashed before it ever
// touches the store; a one-time account-recovery code is minted and returned
// exactly this once (see TODOS.md C422 note on the deferred email-based
// ResetPassword path).
func (s *authServer) Register(ctx context.Context, req backendrpc.RegisterRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "username and password are required")
	}
	// Register is an unauthenticated door with no other size check ahead of it
	// (the wasm client's own validation in internal/app/authcredentials.go is
	// bypassable by calling this RPC directly) — without a server-side cap, a
	// caller could store an arbitrarily large username (bounded only by
	// cfg.GRPCReadLimitBytes, 16MB by default) in every users-table row that
	// references it.
	if len(username) > maxUsernameLength {
		return backendrpc.TokenPairResponse{}, status.Errorf(codes.InvalidArgument, "username must be at most %d characters", maxUsernameLength)
	}
	deviceKey := strings.TrimSpace(req.DeviceLabel)
	if deviceKey == "" {
		deviceKey = "unlabeled-device"
	}
	now := time.Now().UTC()
	if !s.registerLimiter.allow(deviceKey, now) || !s.registerGlobalLimiter.allow(registerGlobalLimiterKey, now) {
		return backendrpc.TokenPairResponse{}, status.Error(codes.ResourceExhausted, "too many registration attempts — try again in a minute")
	}
	if len(req.Password) < minLocalPasswordLength {
		return backendrpc.TokenPairResponse{}, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minLocalPasswordLength)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "password hashing failed")
	}
	recoveryCode, err := generateRecoveryCode()
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "recovery code generation failed")
	}
	recoveryHash, err := bcrypt.GenerateFromPassword([]byte(recoveryCode), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "recovery code hashing failed")
	}
	user, err := s.store.CreateLocalUser(username, string(passwordHash), string(recoveryHash), now)
	if err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			return backendrpc.TokenPairResponse{}, status.Error(codes.AlreadyExists, "username is already registered")
		}
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account creation failed")
	}
	access, refresh, familyID, err := s.issueSession(user.ID, now, req.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, user.ID, "auth.register", "user", user.ID)
	return backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
		RecoveryCode:     recoveryCode,
	}, nil
}

// Login authenticates an existing username/password account.
func (s *authServer) Login(ctx context.Context, req backendrpc.LoginRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "username and password are required")
	}
	if !s.loginLimiter.allow(username, time.Now().UTC()) {
		return backendrpc.TokenPairResponse{}, status.Error(codes.ResourceExhausted, "too many login attempts — try again in a minute")
	}
	user, passwordHash, ok, err := s.store.GetLocalUserByUsername(username)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	// Always run a bcrypt comparison, even when the username doesn't exist,
	// comparing against a fixed dummy hash in that case. Without this, the
	// `!ok` branch returned immediately while a real username paid the full
	// ~bcrypt-cost-10 comparison, producing a wide, measurable response-time
	// gap an unauthenticated caller could use to enumerate valid usernames —
	// the error message is identical either way, but the timing was not.
	compareHash := []byte(passwordHash)
	if !ok {
		compareHash = dummyLoginPasswordHash
	}
	passwordMatches := bcrypt.CompareHashAndPassword(compareHash, []byte(req.Password)) == nil
	if !ok || !passwordMatches {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "username or password is incorrect")
	}
	// A client retry of Login after a timeout — where it can't know whether
	// the first attempt actually landed — must not mint a second device
	// session for one login action (TODOS.md C443). Checked only after the
	// credential check above, so a bad idempotency key never substitutes for
	// a correct password.
	route := backendrpc.MethodAuthLogin
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	requestHash := billingRequestHash(route, user.ID, req.DeviceLabel)
	if idempotencyKey != "" {
		cached, found, err := s.store.GetIdempotencyResult(user.ID, route, idempotencyKey)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency lookup failed")
		}
		if found {
			if cached.RequestHash != requestHash {
				return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "idempotency key was used for a different request")
			}
			var replay backendrpc.TokenPairResponse
			if err := json.Unmarshal(cached.ResponseBody, &replay); err != nil {
				return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency replay decode failed")
			}
			return replay, nil
		}
	}
	now := time.Now().UTC()
	if suspended, err := s.store.IsUserSuspended(user.ID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "suspension check failed")
	} else if suspended {
		return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "account is suspended")
	}
	access, refresh, familyID, err := s.issueSession(user.ID, now, req.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, user.ID, "auth.login", "user", user.ID)
	out := backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
	}
	if idempotencyKey != "" {
		body, err := json.Marshal(out)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "encode login response failed")
		}
		if err := s.store.PutIdempotencyResult(IdempotencyResult{
			UserID:       user.ID,
			Route:        route,
			Key:          idempotencyKey,
			RequestHash:  requestHash,
			ResponseBody: body,
			CreatedAt:    now,
		}); err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency store failed")
		}
	}
	return out, nil
}

// ResetPassword consumes the account's current one-time recovery code. The
// response deliberately matches Register: it starts a fresh session and shows
// a replacement recovery code exactly once. Unknown usernames, wrong codes,
// and already-used codes all take the same bcrypt path and return the same
// error so this unauthenticated door cannot enumerate accounts.
func (s *authServer) ResetPassword(ctx context.Context, req backendrpc.ResetPasswordRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	username := strings.TrimSpace(req.Username)
	recoveryCode := strings.TrimSpace(req.RecoveryCode)
	if username == "" || recoveryCode == "" || req.NewPassword == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "username, recovery code, and new password are required")
	}
	if len(username) > maxUsernameLength {
		return backendrpc.TokenPairResponse{}, status.Errorf(codes.InvalidArgument, "username must be at most %d characters", maxUsernameLength)
	}
	if len(req.NewPassword) < minLocalPasswordLength {
		return backendrpc.TokenPairResponse{}, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minLocalPasswordLength)
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if len(idempotencyKey) > maxIdempotencyKeyLength {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "idempotency key is too long")
	}
	now := time.Now().UTC()
	if !s.recoveryLimiter.allow(username, now) || !s.recoveryGlobalLimiter.allow(recoveryGlobalLimiterKey, now) {
		return backendrpc.TokenPairResponse{}, status.Error(codes.ResourceExhausted, "too many recovery attempts — try again in a minute")
	}
	user, recoveryHash, ok, err := s.store.GetLocalRecoveryByUsername(username)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account recovery failed")
	}
	route := backendrpc.MethodAuthResetPassword
	requestHash := billingRequestHash(route, username, recoveryCode, req.NewPassword, req.DeviceLabel)
	if ok && idempotencyKey != "" {
		cached, found, err := s.store.GetIdempotencyResult(user.ID, route, idempotencyKey)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency lookup failed")
		}
		if found {
			if cached.RequestHash != requestHash {
				return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "idempotency key was used for a different request")
			}
			var replay backendrpc.TokenPairResponse
			if err := json.Unmarshal(cached.ResponseBody, &replay); err != nil {
				return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency replay decode failed")
			}
			replay.RecoveryCode = deterministicRecoveryCode(s.cfg.SessionKey, user.ID, idempotencyKey)
			return replay, nil
		}
	}
	compareHash := []byte(recoveryHash)
	if !ok {
		compareHash = dummyRecoveryCodeHash
	}
	recoveryMatches := bcrypt.CompareHashAndPassword(compareHash, []byte(recoveryCode)) == nil
	if !ok || !recoveryMatches {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "username or recovery code is incorrect")
	}
	if suspended, err := s.store.IsUserSuspended(user.ID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "suspension check failed")
	} else if suspended {
		return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "account is suspended")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "password hashing failed")
	}
	var replacementCode string
	if idempotencyKey != "" {
		replacementCode = deterministicRecoveryCode(s.cfg.SessionKey, user.ID, idempotencyKey)
	} else {
		replacementCode, err = generateRecoveryCode()
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "recovery code generation failed")
		}
	}
	replacementHash, err := bcrypt.GenerateFromPassword([]byte(replacementCode), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "recovery code hashing failed")
	}
	rotated, err := s.store.RotateLocalRecovery(user.ID, recoveryHash, string(passwordHash), string(replacementHash), now)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account recovery failed")
	}
	if !rotated {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "username or recovery code is incorrect")
	}
	access, refresh, familyID, err := s.issueSession(user.ID, now, req.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, user.ID, "auth.password.reset", "user", user.ID)
	out := backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
		RecoveryCode:     replacementCode,
	}
	if idempotencyKey != "" {
		cached := out
		cached.RecoveryCode = ""
		body, err := json.Marshal(cached)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "encode recovery response failed")
		}
		if err := s.store.PutIdempotencyResult(IdempotencyResult{
			UserID:       user.ID,
			Route:        route,
			Key:          idempotencyKey,
			RequestHash:  requestHash,
			ResponseBody: body,
			CreatedAt:    now,
		}); err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "idempotency store failed")
		}
	}
	return out, nil
}

// deterministicRecoveryCode makes an idempotent ResetPassword retry return
// the same replacement code without persisting that plaintext secret. The
// server session key is the HMAC key; the database retains only bcrypt's hash
// and a cached token pair with RecoveryCode redacted.
func deterministicRecoveryCode(sessionKey, userID, idempotencyKey string) string {
	mac := hmac.New(sha256.New, []byte(sessionKey))
	_, _ = mac.Write([]byte(backendrpc.MethodAuthResetPassword))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(idempotencyKey))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(mac.Sum(nil)[:10])
}

// RequestDevicePairing starts the admin-approved device-pairing bootstrap
// (TODOS.md C454): an unauthenticated device — with no working credentials
// yet — asks to be paired, and gets back an opaque id to watch
// (WatchPairingStatus) or cancel (CancelDevicePairing). No account exists or
// is created here; this is entirely BEFORE the account layer.
func (s *authServer) RequestDevicePairing(ctx context.Context, req backendrpc.RequestDevicePairingRequest) (backendrpc.RequestDevicePairingResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.RequestDevicePairingResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	label := strings.TrimSpace(req.DeviceLabel)
	if label == "" {
		label = "unlabeled-device"
	}
	if len(label) > maxUsernameLength {
		return backendrpc.RequestDevicePairingResponse{}, status.Errorf(codes.InvalidArgument, "device label must be at most %d characters", maxUsernameLength)
	}
	now := time.Now().UTC()
	if !s.devicePairingLimiter.allow(label, now) || !s.devicePairingGlobalLimiter.allow(devicePairingGlobalLimiterKey, now) {
		return backendrpc.RequestDevicePairingResponse{}, status.Error(codes.ResourceExhausted, "too many pairing requests — try again in a minute")
	}
	deviceID, _, err := s.store.MintPendingDevice(label, now)
	if err != nil {
		return backendrpc.RequestDevicePairingResponse{}, status.Error(codes.Internal, "pending device request failed")
	}
	return backendrpc.RequestDevicePairingResponse{DeviceID: deviceID}, nil
}

// WatchPairingStatusRPC streams exactly one PairingStatusEvent for a pending
// device request, then closes (TODOS.md C454) — a one-shot watch, not a
// resumable subscription: the device is expected to hold this stream open
// for the lifetime of one "waiting for approval" screen, matching the
// product decision that a pending request doesn't survive a page reload.
// Deliberately polls the store on a short interval rather than a pub/sub
// broadcast (contrast SyncService.subscribeWorkspaces, a high-fanout,
// low-latency path): this is a rare, human-paced flow (someone has to
// notice and click Approve on an admin screen), where a poll interval
// imperceptible to a human is far simpler than a subscription registry
// built for a fundamentally different traffic shape.
func (s *authServer) WatchPairingStatusRPC(req backendrpc.WatchPairingStatusRequest, stream grpc.ServerStream) error {
	if s == nil || s.store == nil {
		return status.Error(codes.FailedPrecondition, "store is not configured")
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return status.Error(codes.InvalidArgument, "device id is required")
	}
	now := time.Now().UTC()
	if !s.watchPairingStatusLimiter.allow(deviceID, now) || !s.watchPairingStatusGlobalLimiter.allow(watchPairingStatusGlobalLimiterKey, now) {
		return status.Error(codes.ResourceExhausted, "too many watch requests — try again in a minute")
	}
	const pollInterval = time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			pd, ok, err := s.store.GetPendingDevice(deviceID)
			if err != nil {
				return status.Error(codes.Internal, "pending device lookup failed")
			}
			if !ok {
				return status.Error(codes.NotFound, "pending device request not found")
			}
			if pd.Status == PendingDeviceStatusPending {
				if time.Now().UTC().Before(pd.ExpiresAt) {
					continue
				}
				_ = stream.SendMsg(&backendrpc.PairingStatusEvent{Status: "expired"})
				return nil
			}
			ev := backendrpc.PairingStatusEvent{Status: pd.Status}
			// A REDEEMED request carries its code too. A device that reconnects
			// this watch after redeeming would otherwise be told "redeemed" with
			// nothing to redeem, and would sit waiting for a code that never
			// comes. The code is single-use and already consumed, so re-sending
			// it grants nothing new — RedeemPairingCode replays the cached token
			// pair for exactly this case (C443).
			if pd.Status == PendingDeviceStatusApproved || pd.Status == PendingDeviceStatusRedeemed {
				ev.PairingCode = pd.PairingCode
			}
			if err := stream.SendMsg(&ev); err != nil {
				return err
			}
			return nil
		}
	}
}

// CancelDevicePairing lets the requesting device withdraw its own pending
// request (TODOS.md C454) — e.g. the user changed their mind, or the pairing
// code WatchPairingStatus pushed doesn't match what the admin console shows
// (a plausible sign of a mismatched or spoofed request, worth killing
// immediately rather than leaving outstanding). Unauthenticated by design:
// possession of deviceID (an unguessable id only ever returned to the
// requesting device itself — see pendingDeviceIDBytes) is the only
// credential this needs, the same trust model RequestDevicePairing already
// established. Succeeds identically whether the ADMIN or the DEVICE rejects
// a request, though they are now recorded distinctly (C700).
func (s *authServer) CancelDevicePairing(ctx context.Context, req backendrpc.CancelDevicePairingRequest) (backendrpc.CancelDevicePairingResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.CancelDevicePairingResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return backendrpc.CancelDevicePairingResponse{}, status.Error(codes.InvalidArgument, "device id is required")
	}
	now := time.Now().UTC()
	if !s.cancelDevicePairingLimiter.allow(deviceID, now) || !s.cancelDevicePairingGlobalLimiter.allow(cancelDevicePairingGlobalLimiterKey, now) {
		return backendrpc.CancelDevicePairingResponse{}, status.Error(codes.ResourceExhausted, "too many cancel requests — try again in a minute")
	}
	// C700: a device withdrawing its own request is recorded as CANCELED, not
	// as an operator rejection. Same authority, different fact.
	canceled, err := s.store.CancelPendingDevice(deviceID)
	if err != nil {
		return backendrpc.CancelDevicePairingResponse{}, status.Error(codes.Internal, "cancel pending device failed")
	}
	return backendrpc.CancelDevicePairingResponse{Canceled: canceled}, nil
}

// SetPassword attaches a username/password to the CALLER's own authenticated
// session (AuthUserFromContext) — never a new account (TODOS.md C454). This
// is the second half of the pairing bootstrap: RedeemPairingCode establishes
// a session for whatever account the pairing code was minted against, and
// SetPassword is how that session, on its first visit, turns into a normal
// username/password login for every subsequent visit.
//
// Deliberately a distinct RPC from Register, not a reuse of it: Register
// (see its doc comment) never checks caller auth state and always mints a
// BRAND-NEW account — calling it after RedeemPairingCode would silently
// create a second, disconnected account instead of attaching credentials to
// the one pairing just granted. The caller's user row is lazily
// materialized first (ensureUserRow — see its doc comment) since a
// token-mode session's row may not exist yet.
func (s *authServer) SetPassword(ctx context.Context, req backendrpc.SetPasswordRequest) (backendrpc.SetPasswordResponse, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if s == nil || s.store == nil {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.InvalidArgument, "username and password are required")
	}
	if len(username) > maxUsernameLength {
		return backendrpc.SetPasswordResponse{}, status.Errorf(codes.InvalidArgument, "username must be at most %d characters", maxUsernameLength)
	}
	if len(req.Password) < minLocalPasswordLength {
		return backendrpc.SetPasswordResponse{}, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minLocalPasswordLength)
	}
	// Authenticated already (AuthUserFromContext above resolved a valid
	// session), so no separate rate limiter — unlike Register/Login/
	// RequestDevicePairing, this door cannot be hit by an unauthenticated
	// attacker at all.
	if err := ensureUserRow(s.store, user); err != nil {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "password hashing failed")
	}
	recoveryCode, err := generateRecoveryCode()
	if err != nil {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "recovery code generation failed")
	}
	recoveryHash, err := bcrypt.GenerateFromPassword([]byte(recoveryCode), bcrypt.DefaultCost)
	if err != nil {
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "recovery code hashing failed")
	}
	if err := s.store.SetLocalCredentials(user.ID, username, string(passwordHash), string(recoveryHash)); err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			return backendrpc.SetPasswordResponse{}, status.Error(codes.AlreadyExists, "username is already registered")
		}
		if errors.Is(err, ErrUserNotFound) {
			return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "account lookup failed")
		}
		return backendrpc.SetPasswordResponse{}, status.Error(codes.Internal, "set password failed")
	}
	s.auditActor(ctx, user.ID, "auth.password.set", "user", user.ID)
	return backendrpc.SetPasswordResponse{RecoveryCode: recoveryCode}, nil
}

// minLocalPasswordLength is the minimum password length Register accepts.
const minLocalPasswordLength = 8

// maxUsernameLength is the maximum username length Register accepts —
// generous for any real login handle, but bounded so an unauthenticated
// caller can't stash an arbitrarily large string (up to the gRPC tunnel's
// read limit) in every row that references a user id.
const maxUsernameLength = 128

// dummyLoginPasswordHash is a bcrypt hash of a fixed, never-issued placeholder
// password, computed once at package init. Login compares an unknown
// username's submitted password against this hash (see the comment at its
// call site) purely to burn the same wall-clock time bcrypt.CompareHashAndPassword
// would take for a real account — it can never itself authenticate anyone.
var dummyLoginPasswordHash = mustBcryptHash("cashflux-login-timing-mitigation-placeholder")

var dummyRecoveryCodeHash = mustBcryptHash("cashflux-recovery-timing-mitigation-placeholder")

// mustBcryptHash hashes password at bcrypt.DefaultCost, the same cost every
// real account's password/recovery-code hash uses (see Register), and panics
// on failure — this only ever runs once, at package init, against a fixed
// input, so an error here means the process's crypto/rand source is broken,
// not that the input was bad.
func mustBcryptHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("server: precompute dummy bcrypt hash: %v", err))
	}
	return hash
}

// issueSession mints a brand-new refresh session family and access/refresh
// pair for userID — the shared plumbing behind Register/Login/
// RedeemPairingCode, each of which is a distinct front door onto the same
// issueStoredSessionPair primitive RefreshToken already uses.
func (s *authServer) issueSession(userID string, now time.Time, deviceLabel string) (access, refresh, familyID string, err error) {
	familyID, err = randomURLToken(24)
	if err != nil {
		return "", "", "", fmt.Errorf("server session: generate refresh family: %w", err)
	}
	access, refresh, err = issueStoredSessionPair(s.cfg, s.store, userID, now, familyID, deviceLabel)
	if err != nil {
		return "", "", "", err
	}
	return access, refresh, familyID, nil
}

// RefreshToken rotates a refresh token for a new access/refresh pair. It is
// the gRPC twin of handleOAuthRefresh (oauth_http.go): single-use consume via
// store.ConsumeRefreshSession, reuse of an already-rotated token revokes the
// whole session family (compromise signal, TODOS.md C423), and a suspended
// account is denied even with a still-valid refresh token.
func (s *authServer) RefreshToken(ctx context.Context, req backendrpc.RefreshTokenRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "refresh token is required")
	}
	now := time.Now().UTC()
	claims, ok := verifySessionClaims(s.cfg, refreshToken, "refresh", now)
	if !ok || strings.TrimSpace(claims.JTI) == "" || strings.TrimSpace(claims.Family) == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "refresh token is invalid")
	}
	session, ok, err := s.store.ConsumeRefreshSession(claims.JTI, sessionTokenHash(refreshToken), now)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "refresh token lookup failed")
	}
	if !ok {
		if strings.TrimSpace(session.FamilyID) != "" {
			_ = s.store.RevokeRefreshSessionFamily(session.FamilyID, now)
			s.auditActor(ctx, session.UserID, "auth.token.reuse", "session_family", session.FamilyID)
		}
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "refresh token is invalid")
	}
	if suspended, err := s.store.IsUserSuspended(session.UserID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "suspension check failed")
	} else if suspended {
		_ = s.store.RevokeRefreshSessionFamily(session.FamilyID, now)
		s.auditActor(ctx, session.UserID, "auth.token.refresh.suspended", "user", session.UserID)
		return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "account is suspended")
	}
	access, refresh, err := issueStoredSessionPair(s.cfg, s.store, session.UserID, now, session.FamilyID, session.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, session.UserID, "auth.token.refresh", "user", session.UserID)
	return backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         session.FamilyID,
	}, nil
}

// Logout revokes the session family the given refresh token belongs to. It is
// idempotent: an already-invalid/expired/unknown refresh token has nothing
// left to revoke, so it reports Revoked: false rather than erroring — the
// caller's goal ("stop trusting this token") is already satisfied.
func (s *authServer) Logout(ctx context.Context, req backendrpc.LogoutRequest) (backendrpc.LogoutResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.LogoutResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return backendrpc.LogoutResponse{}, status.Error(codes.InvalidArgument, "refresh token is required")
	}
	now := time.Now().UTC()
	claims, ok := verifySessionClaims(s.cfg, refreshToken, "refresh", now)
	if !ok || strings.TrimSpace(claims.Family) == "" {
		return backendrpc.LogoutResponse{Revoked: false}, nil
	}
	revoked, err := s.store.RevokeRefreshSessionFamilyForUser(claims.Sub, claims.Family, now)
	if err != nil {
		return backendrpc.LogoutResponse{}, status.Error(codes.Internal, "session revoke failed")
	}
	if revoked {
		s.auditActor(ctx, claims.Sub, "auth.logout", "session_family", claims.Family)
	}
	return backendrpc.LogoutResponse{Revoked: revoked}, nil
}

// ListDevices returns the caller's active device/session list — a thin
// wrapper around Store.ListRefreshSessionFamilies, the same primitive the
// existing OAuth HTTP session-list endpoint uses (handleOAuthListSessions).
//
// Unlike that HTTP endpoint, this gRPC call cannot mark which entry is the
// "current" device: AuthUnaryInterceptor authenticates gRPC calls off the
// short-lived ACCESS token (see grpcTokenValidator), which carries no
// JTI/family claim — only refresh tokens do. Determining "current" would
// require the client to also present its own family id, which the proto
// doesn't carry on this request; leaving Current unset here is deliberate,
// not an oversight.
func (s *authServer) ListDevices(ctx context.Context, _ backendrpc.ListDevicesRequest) (backendrpc.ListDevicesResponse, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return backendrpc.ListDevicesResponse{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if s == nil || s.store == nil {
		return backendrpc.ListDevicesResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	families, err := s.store.ListRefreshSessionFamilies(user.ID, time.Now().UTC())
	if err != nil {
		return backendrpc.ListDevicesResponse{}, status.Error(codes.Internal, "device list failed")
	}
	devices := make([]backendrpc.DeviceSession, 0, len(families))
	for _, family := range families {
		devices = append(devices, backendrpc.DeviceSession{
			FamilyID:    family.FamilyID,
			DeviceLabel: family.DeviceLabel,
			ExpiresAt:   formatTime(family.ExpiresAt),
		})
	}
	return backendrpc.ListDevicesResponse{Devices: devices}, nil
}

// RevokeDevice signs one device out by revoking its session family — a thin
// wrapper around Store.RevokeRefreshSessionFamilyForUser, scoped to the
// caller's own user id so one account can never revoke another's session.
func (s *authServer) RevokeDevice(ctx context.Context, req backendrpc.RevokeDeviceRequest) (backendrpc.RevokeDeviceResponse, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return backendrpc.RevokeDeviceResponse{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if s == nil || s.store == nil {
		return backendrpc.RevokeDeviceResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	familyID := strings.TrimSpace(req.FamilyID)
	if familyID == "" {
		return backendrpc.RevokeDeviceResponse{}, status.Error(codes.InvalidArgument, "family id is required")
	}
	revoked, err := s.store.RevokeRefreshSessionFamilyForUser(user.ID, familyID, time.Now().UTC())
	if err != nil {
		return backendrpc.RevokeDeviceResponse{}, status.Error(codes.Internal, "device revoke failed")
	}
	if !revoked {
		return backendrpc.RevokeDeviceResponse{}, status.Error(codes.NotFound, "device not found")
	}
	auditFromContext(ctx, s.store, "auth.session.revoke", "session_family", familyID)
	return backendrpc.RevokeDeviceResponse{Revoked: true}, nil
}

// auditActor appends an audit event for RefreshToken/Logout, which run before
// AuthUnaryInterceptor ever puts an AuthUser in context (see
// authInterceptorSkipMethods) — so, unlike auditFromContext, the actor id must
// be passed explicitly rather than read off the context.
func (s *authServer) auditActor(ctx context.Context, actorID, action, targetType, targetID string) {
	if s == nil || s.store == nil || strings.TrimSpace(actorID) == "" {
		return
	}
	requestID, _ := RequestIDFromContext(ctx)
	_, _ = s.store.AppendAuditEvent(AuditEvent{
		Timestamp:  time.Now().UTC(),
		ActorID:    actorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID,
	})
}

func authEnrollHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.EnrollRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Enroll(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthEnroll}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).Enroll(ctx, req.(backendrpc.EnrollRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authRedeemPairingCodeHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RedeemPairingCodeRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RedeemPairingCode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRedeemPairingCode}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).RedeemPairingCode(ctx, req.(backendrpc.RedeemPairingCodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authRegisterHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RegisterRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRegister}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).Register(ctx, req.(backendrpc.RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authLoginHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.LoginRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthLogin}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).Login(ctx, req.(backendrpc.LoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authResetPasswordHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.ResetPasswordRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).ResetPassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthResetPassword}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).ResetPassword(ctx, req.(backendrpc.ResetPasswordRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authRefreshTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RefreshTokenRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RefreshToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRefreshToken}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).RefreshToken(ctx, req.(backendrpc.RefreshTokenRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authLogoutHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.LogoutRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Logout(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthLogout}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).Logout(ctx, req.(backendrpc.LogoutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authListDevicesHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.ListDevicesRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).ListDevices(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthListDevices}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).ListDevices(ctx, req.(backendrpc.ListDevicesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authRevokeDeviceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RevokeDeviceRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RevokeDevice(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRevokeDevice}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).RevokeDevice(ctx, req.(backendrpc.RevokeDeviceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authRequestDevicePairingHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RequestDevicePairingRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RequestDevicePairing(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRequestDevicePairing}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).RequestDevicePairing(ctx, req.(backendrpc.RequestDevicePairingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authCancelDevicePairingHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.CancelDevicePairingRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).CancelDevicePairing(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthCancelDevicePairing}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).CancelDevicePairing(ctx, req.(backendrpc.CancelDevicePairingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authSetPasswordHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.SetPasswordRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).SetPassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthSetPassword}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).SetPassword(ctx, req.(backendrpc.SetPasswordRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authWatchPairingStatusHandler(srv any, stream grpc.ServerStream) error {
	var in backendrpc.WatchPairingStatusRequest
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return srv.(authServiceServer).WatchPairingStatusRPC(in, stream)
}

// IssueSessionForUser mints a session pair for a user id directly, with no RPC,
// no credential exchange and no pairing code — the primitive behind an embedding
// host provisioning a session for a caller it has ALREADY authenticated itself.
//
// This deliberately bypasses every front door (Register/Login/RedeemPairingCode)
// because there is nothing left to verify: the host proved the caller's identity
// with its own admin session before calling, and CashFlux has no better evidence
// to ask for. It is unexported-by-convention dangerous — it hands out a full
// session for any user id — so it is reachable only from Go code inside the host
// process, never from any RPC or HTTP surface.
//
// Refuses a suspended account, matching every other path that issues a session:
// suspension that only blocked the RPC doors would be trivially routed around by
// the very shortcut this function provides.
func IssueSessionForUser(cfg Config, store *Store, userID, deviceLabel string) (backendrpc.TokenPairResponse, error) {
	if store == nil {
		return backendrpc.TokenPairResponse{}, fmt.Errorf("server session: store is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return backendrpc.TokenPairResponse{}, fmt.Errorf("server session: user id is required")
	}
	if suspended, err := store.IsUserSuspended(userID); err != nil {
		return backendrpc.TokenPairResponse{}, fmt.Errorf("server session: suspension check: %w", err)
	} else if suspended {
		return backendrpc.TokenPairResponse{}, fmt.Errorf("server session: account is suspended")
	}
	now := time.Now().UTC()
	familyID, err := randomURLToken(24)
	if err != nil {
		return backendrpc.TokenPairResponse{}, fmt.Errorf("server session: generate refresh family: %w", err)
	}
	access, refresh, err := issueStoredSessionPair(cfg, store, userID, now, familyID, deviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, err
	}
	return backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
	}, nil
}
