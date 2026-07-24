// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestAuthServerRedeemPairingCodeSurvivesDeviceLabelRotation is an adversarial
// proof for the 2026-07 Y-wave auth security review: pairingLimiter is keyed
// ONLY by the caller-supplied, unauthenticated DeviceLabel (see
// authserver.pairingLimiter's doc comment). An attacker who rotates the
// DeviceLabel on every call therefore never shares a bucket with a previous
// attempt and must NOT be able to brute force a 6-digit / 5-minute-TTL
// pairing code (1,000,000 possibilities) at an unbounded rate — that would be
// a full unauthenticated account takeover (RedeemPairingCode mints a real
// session for the target account with no other credential check). This test
// sends far more attempts than pairingLimitPerMinute allows, each with a
// distinct device label, and asserts the caller is still throttled.
func TestAuthServerRedeemPairingCodeSurvivesDeviceLabelRotation(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	const attempts = pairingLimitPerMinute * 5 // comfortably more than any single-device bucket would allow
	throttled := false
	for i := 0; i < attempts; i++ {
		_, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{
			PairingCode: "000000",
			DeviceLabel: "attacker-device-" + strconv.Itoa(i), // a fresh, unauthenticated label every call
		})
		if status.Code(err) == codes.ResourceExhausted {
			throttled = true
			break
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("call %d: unexpected error %v", i, err)
		}
	}
	if !throttled {
		t.Fatalf("RedeemPairingCode: %d guesses across %d rotated device labels were never throttled — "+
			"an attacker can brute force a 6-digit pairing code (1,000,000 combinations) within its "+
			"5-minute TTL by rotating DeviceLabel per request, since pairingLimiter has no non-bypassable "+
			"(global or per-code) backstop", attempts, attempts)
	}
}

// TestAuthServerRegisterSurvivesDeviceLabelRotation is a regression guard for
// a bypass this review found and fixed: registerLimiter alone is keyed solely
// on the caller-supplied, unauthenticated DeviceLabel, so an attacker
// rotating it on every call never shared a per-device bucket with their own
// previous attempts and got unlimited, unthrottled Register calls — each
// doing two bcrypt.DefaultCost hashes (password + one-time recovery code), a
// real CPU-exhaustion / account-spam DoS vector. registerGlobalLimiter (a
// single un-keyed, un-bypassable bucket, mirroring pairingGlobalLimiter) now
// backstops it; this test proves a caller rotating DeviceLabel past that
// global ceiling still gets throttled.
func TestAuthServerRegisterSurvivesDeviceLabelRotation(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	const attempts = registerGlobalLimitPerMinute + 5
	throttled := false
	for i := 0; i < attempts; i++ {
		req := backendrpc.RegisterRequest{
			Username:    "spam-user-" + strconv.Itoa(i),
			Password:    "correct-horse-battery",
			DeviceLabel: "attacker-device-" + strconv.Itoa(i), // a fresh, unauthenticated label every call
		}
		_, err := s.Register(ctx, req)
		if status.Code(err) == codes.ResourceExhausted {
			throttled = true
			break
		}
		if err != nil {
			t.Fatalf("call %d: unexpected error %v", i, err)
		}
	}
	if !throttled {
		t.Fatalf("Register: %d account-creation calls across %d rotated device labels were never throttled — "+
			"registerLimiter has no non-bypassable (global) backstop the way pairingLimiter now does, so an "+
			"attacker can spam accounts / burn CPU on bcrypt hashing at an unbounded rate by rotating DeviceLabel", attempts, attempts)
	}
}

// TestRefreshTokenRejectsAccessTokenTypeConfusion proves an access token
// (short-lived, no JTI/Family claims) cannot be replayed as a refresh token —
// i.e. sessionClaims.Type is actually enforced on the RefreshToken path, not
// merely parsed. A missing check here would let a caller mint an endless
// stream of "refreshed" sessions directly from a still-valid access token,
// well past the access token's intended lifetime, without ever needing the
// real single-use refresh token.
func TestRefreshTokenRejectsAccessTokenTypeConfusion(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	now := time.Now().UTC()

	access, err := issueSessionToken(s.cfg, "local:cam", "access", sessionAccessTTL, now)
	if err != nil {
		t.Fatalf("issueSessionToken: %v", err)
	}
	if _, err := s.RefreshToken(ctx, backendrpc.RefreshTokenRequest{RefreshToken: access}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("RefreshToken with an access token in place of a refresh token: err = %v, want Unauthenticated", err)
	}
}

// TestGRPCTokenValidatorRejectsRefreshTokenTypeConfusion proves a refresh
// token cannot be used as a bearer access token against any authenticated
// AuthService/SyncService/etc. call. grpcTokenValidator's oauth-mode branch
// hardcodes tokenType "access" — this pins that behavior against regression.
func TestGRPCTokenValidatorRejectsRefreshTokenTypeConfusion(t *testing.T) {
	now := time.Now().UTC()
	cfg := Config{SessionKey: "test-session-key-0123456789", AuthMode: "oauth"}
	refresh, err := issueSessionToken(cfg, "local:cam", "refresh", sessionRefreshTTL, now)
	if err != nil {
		t.Fatalf("issueSessionToken: %v", err)
	}
	if _, ok := authUserForToken(refresh, cfg); ok {
		t.Fatal("a refresh token validated as a bearer access token — type confusion")
	}
}
