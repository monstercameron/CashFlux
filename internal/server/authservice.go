// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/twilio"
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
func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.AuthService",
		HandlerType: (*AuthServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Enroll", Handler: authEnrollHandler},
			{MethodName: "RequestPhoneVerification", Handler: authRequestPhoneVerificationHandler},
			{MethodName: "VerifyPhoneCode", Handler: authVerifyPhoneCodeHandler},
			{MethodName: "RedeemPairingCode", Handler: authRedeemPairingCodeHandler},
			{MethodName: "Register", Handler: authRegisterHandler},
			{MethodName: "Login", Handler: authLoginHandler},
			{MethodName: "RefreshToken", Handler: authRefreshTokenHandler},
			{MethodName: "Logout", Handler: authLogoutHandler},
			{MethodName: "ListDevices", Handler: authListDevicesHandler},
			{MethodName: "RevokeDevice", Handler: authRevokeDeviceHandler},
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
	RequestPhoneVerification(context.Context, backendrpc.RequestPhoneVerificationRequest) (backendrpc.RequestPhoneVerificationResponse, error)
	VerifyPhoneCode(context.Context, backendrpc.VerifyPhoneCodeRequest) (backendrpc.TokenPairResponse, error)
	RedeemPairingCode(context.Context, backendrpc.RedeemPairingCodeRequest) (backendrpc.TokenPairResponse, error)
	Register(context.Context, backendrpc.RegisterRequest) (backendrpc.TokenPairResponse, error)
	Login(context.Context, backendrpc.LoginRequest) (backendrpc.TokenPairResponse, error)
	RefreshToken(context.Context, backendrpc.RefreshTokenRequest) (backendrpc.TokenPairResponse, error)
	Logout(context.Context, backendrpc.LogoutRequest) (backendrpc.LogoutResponse, error)
	ListDevices(context.Context, backendrpc.ListDevicesRequest) (backendrpc.ListDevicesResponse, error)
	RevokeDevice(context.Context, backendrpc.RevokeDeviceRequest) (backendrpc.RevokeDeviceResponse, error)
}

// authServer implements AuthServiceServer. cfg is required (not just store)
// because RefreshToken/Logout sign and verify session JWTs exactly like the
// existing OAuth HTTP handlers (issueStoredSessionPair/verifySessionClaims,
// both cfg-keyed) — see internal/server/session.go and oauth_http.go.
type authServer struct {
	store *Store
	cfg   Config

	// verify is the Twilio Verify client SMS enrollment calls through (TODOS.md
	// C420). It defaults to nil and is built lazily from cfg on first use
	// (see verifyClient) so newAuthService's signature stays untouched for
	// existing callers; tests set it directly on a struct literal to inject a
	// fake without hitting the network.
	verify twilio.VerifyClient

	// phoneVerifyLimiter/deviceVerifyLimiter throttle RequestPhoneVerification
	// per phone number and per calling device, reusing the same
	// newFixedWindowLimiter primitive as the HTTP authLimiter
	// (authRateLimitMiddleware in http.go) rather than inventing a new limiter.
	// The gRPC tunnel carries no client IP (see BearerTokenFromContext/
	// metadata.FromIncomingContext — only "authorization" and "x-request-id"
	// are set), so "per caller device" is scoped by the request's DeviceLabel,
	// the best caller-identifying signal available at this layer; a request
	// with no label shares one bucket, which still caps a single unlabeled
	// caller's rate. Both are fixed at phoneVerifyLimitPerMinute /
	// deviceVerifyLimitPerMinute (conservative, SMS costs money) rather than
	// cfg.AuthRateLimitPerMinute, which is a general-purpose 20/min default
	// meant for cheap auth endpoints, not paid SMS sends.
	phoneVerifyLimiter  *fixedWindowLimiter
	deviceVerifyLimiter *fixedWindowLimiter

	// checkCodeLimiter/loginLimiter/registerLimiter/pairingLimiter guard the
	// remaining unauthenticated AuthService doors against brute force: a
	// six-digit SMS code, a password, and a six-digit pairing code are all
	// small-enough guess spaces that an unrated endpoint is a real attack
	// surface (account takeover / device hijack), not just a cost concern.
	// Each is keyed by the best caller-identifying signal available at this
	// layer — see phoneVerifyLimiter's doc comment on why the gRPC tunnel
	// gives us no client IP.
	checkCodeLimiter *fixedWindowLimiter
	loginLimiter     *fixedWindowLimiter
	registerLimiter  *fixedWindowLimiter
	pairingLimiter   *fixedWindowLimiter

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

	// phoneVerifyGlobalLimiter backstops phoneVerifyLimiter/deviceVerifyLimiter
	// with a single, server-wide bucket (see pairingGlobalLimiter's doc
	// comment for the same reasoning): both of those are keyed on
	// caller-supplied values (the target phone number and an unauthenticated
	// device label), so an attacker who sprays a fresh, real phone number
	// with a fresh device label on every call never reuses either bucket.
	// RequestPhoneVerification sends a real, money-costing Twilio SMS on
	// every unthrottled call, so an unbounded spray is a direct financial
	// cost, not just an account-security issue — this limiter caps that
	// regardless of how many distinct phones/labels a caller cycles through.
	phoneVerifyGlobalLimiter *fixedWindowLimiter

	// registerGlobalLimiter is registerGlobalLimiter's pairing-code twin:
	// registerLimiter alone is keyed by the same caller-controlled
	// DeviceLabel, so an attacker rotating it on every call gets unlimited
	// Register attempts — and Register does two bcrypt.DefaultCost hashes per
	// call (password + one-time recovery code), making this a real CPU-
	// exhaustion / account-spam surface, not just a cost concern. See
	// pairingGlobalLimiter's doc comment above for the full rationale; the
	// same trade-off (one server-wide guess budget) applies here.
	registerGlobalLimiter *fixedWindowLimiter
}

// pairingGlobalLimiterKey is the single, constant bucket key
// pairingGlobalLimiter uses — deliberately not derived from anything in the
// request, so it cannot be reset by varying request fields.
const pairingGlobalLimiterKey = "redeem-pairing-code:global"

// phoneVerifyGlobalLimiterKey is phoneVerifyGlobalLimiter's single, constant
// bucket key, for the same reason as pairingGlobalLimiterKey.
const phoneVerifyGlobalLimiterKey = "request-phone-verification:global"

// registerGlobalLimiterKey is registerGlobalLimiter's constant bucket key —
// see pairingGlobalLimiterKey.
const registerGlobalLimiterKey = "register:global"

// phoneVerifyLimitPerMinute/deviceVerifyLimitPerMinute cap SMS verification
// sends per phone number / per calling device per minute (TODOS.md C420).
// checkCodeLimitPerMinute/loginLimitPerMinute/registerLimitPerMinute/
// pairingLimitPerMinute cap attempts against the other guessable-secret doors
// (SMS code check, password, pairing code) per phone/username/device.
const (
	phoneVerifyLimitPerMinute  = 3
	deviceVerifyLimitPerMinute = 5
	checkCodeLimitPerMinute    = 10
	loginLimitPerMinute        = 10
	registerLimitPerMinute     = 5
	pairingLimitPerMinute      = 10

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

	// phoneVerifyGlobalLimitPerMinute caps total RequestPhoneVerification
	// sends across EVERY caller/phone/device server-wide (see
	// authServer.phoneVerifyGlobalLimiter's doc comment). Set well above any
	// plausible legitimate concurrent-enrollment burst but low enough that a
	// spray attack costs the attacker time, not the deployment an unbounded
	// Twilio bill.
	phoneVerifyGlobalLimitPerMinute = 30

	// registerGlobalLimitPerMinute caps total Register attempts across EVERY
	// caller server-wide (see authServer.registerGlobalLimiter's doc comment).
	// Set well above any plausible legitimate concurrent-signup burst but low
	// enough that spamming accounts / burning bcrypt CPU costs the attacker
	// time, not the deployment unbounded CPU no matter how many device labels
	// they rotate through.
	registerGlobalLimitPerMinute = 30
)

func newAuthService(store *Store, cfg Config) *authServer {
	return &authServer{
		store:                    store,
		cfg:                      cfg,
		phoneVerifyLimiter:       newFixedWindowLimiter(phoneVerifyLimitPerMinute),
		deviceVerifyLimiter:      newFixedWindowLimiter(deviceVerifyLimitPerMinute),
		checkCodeLimiter:         newFixedWindowLimiter(checkCodeLimitPerMinute),
		loginLimiter:             newFixedWindowLimiter(loginLimitPerMinute),
		registerLimiter:          newFixedWindowLimiter(registerLimitPerMinute),
		pairingLimiter:           newFixedWindowLimiter(pairingLimitPerMinute),
		pairingGlobalLimiter:     newFixedWindowLimiter(pairingGlobalLimitPerMinute),
		phoneVerifyGlobalLimiter: newFixedWindowLimiter(phoneVerifyGlobalLimitPerMinute),
		registerGlobalLimiter:    newFixedWindowLimiter(registerGlobalLimitPerMinute),
	}
}

// verifyClient returns s.verify if a test (or future wiring) set one directly,
// otherwise builds a TwilioVerifyClient from s.cfg's Twilio fields. Twilio's
// client itself fails clearly (twilio.ErrNotConfigured) when those fields are
// empty, so an unconfigured deployment fails loudly rather than silently
// pretending to send or accept a code.
func (s *authServer) verifyClient() twilio.VerifyClient {
	if s.verify != nil {
		return s.verify
	}
	return twilio.NewTwilioVerifyClient(twilio.Config{
		AccountSID:       s.cfg.TwilioAccountSID,
		AuthToken:        s.cfg.TwilioAuthToken,
		VerifyServiceSID: s.cfg.TwilioVerifyServiceSID,
	})
}

// e164Pattern matches E.164 phone numbers: a leading "+", then 8-15 digits
// with no leading zero (ITU-T E.164 §6: max 15 digits total including the
// country code, and a country code never starts with 0).
var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

// normalizePhoneE164 trims whitespace and confirms phone is a valid E.164
// number, returning ("", false) otherwise. CashFlux does not attempt
// locale-aware national-number parsing here — it requires the client to
// collect and submit a full E.164 number (leading "+" and country code),
// which keeps this server-side check simple and unambiguous.
func normalizePhoneE164(phone string) (string, bool) {
	phone = strings.TrimSpace(phone)
	if !e164Pattern.MatchString(phone) {
		return "", false
	}
	return phone, true
}

// requestVerificationDedupeWindow buckets RequestPhoneVerification retries: a
// second call for the same phone within this window is treated as a client
// retry of the same action, not a new send request, and replays the first
// call's result via the idempotency store instead of re-sending a text.
// RequestPhoneVerificationRequest carries no client-supplied idempotency key
// (unlike VerifyPhoneCodeRequest), so the key is derived from the phone number
// and a coarse time bucket instead.
const requestVerificationDedupeWindow = 30 * time.Second

// phoneUserID derives the stable user id for a phone-verified account: the
// same normalized-phone-keyed id RequestPhoneVerification/VerifyPhoneCode
// both use, so a phone number always maps to exactly one account.
func phoneUserID(phone string) string { return "phone:" + phone }

// ensurePhoneUser upserts a placeholder user row for phone before any
// idempotency-store call for that phone. idempotency_keys.user_id has a
// foreign key into users(id) (see serverSchemaV5 in store.go), so a lookup or
// insert for a phone number that has never upserted a user row would fail the
// constraint — this call guarantees the row exists first. Upserting here
// grants no access: the row carries no password/refresh session, so a phone
// number that never completes VerifyPhoneCode can still never sign in.
func (s *authServer) ensurePhoneUser(phone string, now time.Time) error {
	return s.store.UpsertUser(User{ID: phoneUserID(phone), Provider: "phone", Subject: phone, CreatedAt: now})
}

// requestVerificationIdempotencyKey derives a dedupe key for
// RequestPhoneVerification from phone and a coarse time bucket, so retries
// within requestVerificationDedupeWindow of each other collide onto the same
// key and replay the first attempt's result instead of sending a second text.
func requestVerificationIdempotencyKey(phone string, now time.Time) string {
	bucket := now.UTC().Unix() / int64(requestVerificationDedupeWindow/time.Second)
	return billingRequestHash("auth.requestPhoneVerification", phone, fmt.Sprintf("%d", bucket))
}

// Enroll starts a brand-new device/account pairing (the generic entry point
// TODOS.md C419 falls through to when a device has never enrolled before).
// TODO(laneB): see TODOS.md C419 — concrete enrollment doors are Register/
// Login (C422), RedeemPairingCode (C421), and RequestPhoneVerification/
// VerifyPhoneCode (C420).
func (s *authServer) Enroll(context.Context, backendrpc.EnrollRequest) (backendrpc.TokenPairResponse, error) {
	return backendrpc.TokenPairResponse{}, status.Errorf(codes.Unimplemented, "TODO(laneB): see TODOS.md C419")
}

// RequestPhoneVerification sends an SMS verification code via Twilio Verify
// (TODOS.md C420). It is rate-limited per phone number AND per calling device
// (see phoneVerifyLimiter/deviceVerifyLimiter), and de-duplicated so a client
// retry within requestVerificationDedupeWindow replays the first attempt's
// result instead of sending a second text (see
// requestVerificationIdempotencyKey).
func (s *authServer) RequestPhoneVerification(ctx context.Context, req backendrpc.RequestPhoneVerificationRequest) (backendrpc.RequestPhoneVerificationResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	phone, ok := normalizePhoneE164(req.PhoneNumber)
	if !ok {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.InvalidArgument, "phone number must be in international format, e.g. +15551234567")
	}
	now := time.Now().UTC()
	deviceKey := strings.TrimSpace(req.DeviceLabel)
	if deviceKey == "" {
		deviceKey = "unlabeled-device"
	}
	if !s.phoneVerifyLimiter.allow(phone, now) || !s.deviceVerifyLimiter.allow(deviceKey, now) || !s.phoneVerifyGlobalLimiter.allow(phoneVerifyGlobalLimiterKey, now) {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.ResourceExhausted, "too many verification requests — try again in a minute")
	}
	userID := phoneUserID(phone)
	verifiedBefore, err := s.store.PhoneVerifiedBefore(userID)
	if err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	if err := s.ensurePhoneUser(phone, now); err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	// SetupCode gates account CREATION only (TODOS.md portfolio-embedding
	// gate): a phone that has already completed verification once is a
	// returning user signing in on another device, not a new invite, so it
	// skips this check regardless of cfg.SetupCode. Checked here, fail-fast,
	// so a wrong/spent code never costs an SMS — VerifyPhoneCode is what
	// actually consumes it, only on successful verification.
	if !verifiedBefore && s.cfg.SetupCode != "" {
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.SetupCode)), []byte(s.cfg.SetupCode)) != 1 {
			return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.PermissionDenied, "a valid setup code is required to create a new account")
		}
		available, err := s.store.SetupCodeAvailable(s.cfg.SetupCode)
		if err != nil {
			return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "setup code check failed")
		}
		if !available {
			return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.PermissionDenied, "a valid setup code is required to create a new account")
		}
	}
	route := backendrpc.MethodAuthRequestPhoneVerification
	dedupeKey := requestVerificationIdempotencyKey(phone, now)
	requestHash := billingRequestHash(route, phone)
	if cached, found, err := s.store.GetIdempotencyResult(userID, route, dedupeKey); err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "idempotency lookup failed")
	} else if found {
		if cached.RequestHash != requestHash {
			return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.InvalidArgument, "idempotency key was used for a different request")
		}
		var replay backendrpc.RequestPhoneVerificationResponse
		if err := json.Unmarshal(cached.ResponseBody, &replay); err != nil {
			return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "idempotency replay decode failed")
		}
		return replay, nil
	}
	if err := s.verifyClient().SendCode(ctx, phone); err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Unavailable, "sending the verification code failed")
	}
	out := backendrpc.RequestPhoneVerificationResponse{Sent: true}
	body, err := json.Marshal(out)
	if err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "encode verification response failed")
	}
	if err := s.store.PutIdempotencyResult(IdempotencyResult{
		UserID:       userID,
		Route:        route,
		Key:          dedupeKey,
		RequestHash:  requestHash,
		ResponseBody: body,
		CreatedAt:    now,
	}); err != nil {
		return backendrpc.RequestPhoneVerificationResponse{}, status.Error(codes.Internal, "idempotency store failed")
	}
	return out, nil
}

// VerifyPhoneCode completes SMS enrollment with the code the user received
// (TODOS.md C420). On a correct code it looks up or creates a User keyed by
// the normalized phone number, mints a session with issueSession (the same
// primitive Register/Login/RedeemPairingCode use), and returns a real
// TokenPairResponse. A caller-supplied IdempotencyKey makes a retried verify
// return the SAME token pair rather than minting a second device session
// (TODOS.md C443) — required here because, unlike an ordinary failed request,
// a successful VerifyPhoneCode has an external side effect (Twilio marks the
// code consumed) that must not run twice for one user action.
func (s *authServer) VerifyPhoneCode(ctx context.Context, req backendrpc.VerifyPhoneCodeRequest) (backendrpc.TokenPairResponse, error) {
	if s == nil || s.store == nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	phone, ok := normalizePhoneE164(req.PhoneNumber)
	if !ok {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "phone number must be in international format, e.g. +15551234567")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return backendrpc.TokenPairResponse{}, status.Error(codes.InvalidArgument, "verification code is required")
	}
	now := time.Now().UTC()
	deviceKey := strings.TrimSpace(req.DeviceLabel)
	if deviceKey == "" {
		deviceKey = "unlabeled-device"
	}
	if !s.checkCodeLimiter.allow(phone, now) || !s.deviceVerifyLimiter.allow(deviceKey, now) {
		return backendrpc.TokenPairResponse{}, status.Error(codes.ResourceExhausted, "too many verification attempts — try again in a minute")
	}
	userID := phoneUserID(phone)
	verifiedBefore, err := s.store.PhoneVerifiedBefore(userID)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	if err := s.ensurePhoneUser(phone, now); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "account lookup failed")
	}
	route := backendrpc.MethodAuthVerifyPhoneCode
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	requestHash := billingRequestHash(route, phone, code, req.DeviceLabel)
	if idempotencyKey != "" {
		cached, found, err := s.store.GetIdempotencyResult(userID, route, idempotencyKey)
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
	approved, err := s.verifyClient().CheckCode(ctx, phone, code)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unavailable, "checking the verification code failed")
	}
	if !approved {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Unauthenticated, "verification code is incorrect or expired")
	}
	if suspended, err := s.store.IsUserSuspended(userID); err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "suspension check failed")
	} else if suspended {
		return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "account is suspended")
	}
	// SetupCode's authoritative check-and-consume happens here, only on a
	// verified new account, only after the SMS code itself has already been
	// proven correct — so a fumbled verification attempt never burns the
	// invite (see RequestPhoneVerification's fail-fast check for the same
	// gate, and migrateTo11's doc comment for why verifiedBefore, not user-row
	// existence, is the "new account" signal).
	if !verifiedBefore && s.cfg.SetupCode != "" {
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.SetupCode)), []byte(s.cfg.SetupCode)) != 1 {
			return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "a valid setup code is required to create a new account")
		}
		consumed, err := s.store.ConsumeSetupCode(s.cfg.SetupCode, now)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "setup code consume failed")
		}
		if !consumed {
			return backendrpc.TokenPairResponse{}, status.Error(codes.PermissionDenied, "a valid setup code is required to create a new account")
		}
	}
	if !verifiedBefore {
		if err := s.store.MarkPhoneVerified(userID, now); err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "mark verified failed")
		}
	}
	access, refresh, familyID, err := s.issueSession(userID, now, req.DeviceLabel)
	if err != nil {
		return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "session issue failed")
	}
	s.auditActor(ctx, userID, "auth.phone.verify", "user", userID)
	out := backendrpc.TokenPairResponse{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(sessionAccessTTL.Seconds()),
		DeviceID:         familyID,
	}
	if idempotencyKey != "" {
		body, err := json.Marshal(out)
		if err != nil {
			return backendrpc.TokenPairResponse{}, status.Error(codes.Internal, "encode verification response failed")
		}
		if err := s.store.PutIdempotencyResult(IdempotencyResult{
			UserID:       userID,
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

func authRequestPhoneVerificationHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.RequestPhoneVerificationRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RequestPhoneVerification(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthRequestPhoneVerification}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).RequestPhoneVerification(ctx, req.(backendrpc.RequestPhoneVerificationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func authVerifyPhoneCodeHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.VerifyPhoneCodeRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).VerifyPhoneCode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAuthVerifyPhoneCode}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).VerifyPhoneCode(ctx, req.(backendrpc.VerifyPhoneCodeRequest))
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
