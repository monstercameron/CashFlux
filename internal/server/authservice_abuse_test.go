// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestAuthServerRedeemPairingCodeDeviceLabelRotationDoesNotBypassRateLimit
// proves that RedeemPairingCode's brute-force defense cannot be defeated by
// simply varying the caller-supplied, wholly untrusted DeviceLabel on every
// guess. pairingLimiter alone is keyed ONLY by DeviceLabel (see
// authServer.RedeemPairingCode), and DeviceLabel is an arbitrary
// client-supplied string with no verification behind it — an attacker who
// sends a fresh random label on every call was, before this test's
// accompanying fix, able to guess pairing codes at an effectively unbounded
// rate, turning a "1-in-a-million over a 5-minute TTL" secret (see
// PairingCodeTTL/pairingCodeDigits in pairingcode.go) into a practically
// brute-forceable one and taking over the target account.
func TestAuthServerRedeemPairingCodeDeviceLabelRotationDoesNotBypassRateLimit(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	// Attempt far more than any single-device limit while using a brand-new,
	// never-reused DeviceLabel on every call — exactly what a real attacker
	// controls and would do to dodge a per-device bucket.
	const attempts = pairingLimitPerMinute * 5
	exhausted := 0
	for i := 0; i < attempts; i++ {
		_, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{
			PairingCode: "000000",
			DeviceLabel: fmt.Sprintf("attacker-device-%d", i),
		})
		if err == nil {
			t.Fatalf("call %d: unexpected success guessing an unminted code", i)
		}
		if codeOf(err) == codes.ResourceExhausted {
			exhausted++
		}
	}
	if exhausted == 0 {
		t.Fatalf("RedeemPairingCode: %d guesses with a fresh DeviceLabel every time, none throttled — "+
			"the rate limit is fully bypassable by an attacker who rotates the (client-controlled, "+
			"unverified) device label on every attempt, defeating brute-force protection on a "+
			"6-digit/5-minute-TTL account-takeover secret", attempts)
	}
}

// TestAuthServerRegisterDeviceLabelRotationDoesNotBypassRateLimit is the same
// bypass shape as the pairing-code test above, applied to Register: an
// attacker who mints a fresh DeviceLabel per call must not be able to spam
// account creation (and the two bcrypt.DefaultCost hashes Register does per
// call) past registerLimitPerMinute by dodging the per-device bucket.
func TestAuthServerRegisterDeviceLabelRotationDoesNotBypassRateLimit(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	// Must comfortably clear whichever backstop is in place — a per-device
	// bucket alone would trip well before registerLimitPerMinute*5, but a
	// server-wide backstop (registerGlobalLimitPerMinute) needs more calls
	// than that to prove it actually engages.
	const attempts = registerGlobalLimitPerMinute + 5
	exhausted := 0
	for i := 0; i < attempts; i++ {
		req := backendrpc.RegisterRequest{
			Username:    fmt.Sprintf("spam-user-%d", i),
			Password:    "correct-horse-battery",
			DeviceLabel: fmt.Sprintf("attacker-device-%d", i),
		}
		_, err := s.Register(ctx, req)
		if err != nil && codeOf(err) == codes.ResourceExhausted {
			exhausted++
		}
	}
	if exhausted == 0 {
		t.Fatalf("Register: %d account-creation calls with a fresh DeviceLabel every time, none throttled — "+
			"the rate limit is fully bypassable by an attacker who rotates the (client-controlled, "+
			"unverified) device label on every attempt", attempts)
	}
}

// TestAuthServerLoginTimingDoesNotLeakUsernameExistence proves Login takes
// roughly the same wall-clock time whether the username doesn't exist at all
// or exists with a wrong password. Before the accompanying fix, an unknown
// username short-circuited before ever calling bcrypt.CompareHashAndPassword
// (see the `!ok ||` short-circuit in authServer.Login), while a known
// username with a wrong password always paid the ~bcrypt-cost-10 comparison
// — a wide, measurable timing gap an attacker can use to enumerate valid
// usernames against an endpoint whose error message is otherwise identical
// for both cases.
func TestAuthServerLoginTimingDoesNotLeakUsernameExistence(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	const trials = 8
	knownUsernames := make([]string, trials)
	for i := range knownUsernames {
		knownUsernames[i] = fmt.Sprintf("timing-known-%d", i)
		if _, err := s.Register(ctx, backendrpc.RegisterRequest{
			Username: knownUsernames[i], Password: "correct-horse-battery",
			// A distinct DeviceLabel per setup call so this setup loop itself
			// doesn't trip registerLimiter (keyed by DeviceLabel) once trials
			// exceeds registerLimitPerMinute.
			DeviceLabel: fmt.Sprintf("timing-setup-device-%d", i),
		}); err != nil {
			t.Fatalf("Register setup %d: %v", i, err)
		}
	}

	var knownElapsed, unknownElapsed time.Duration
	for i := 0; i < trials; i++ {
		start := time.Now()
		_, _ = s.Login(ctx, backendrpc.LoginRequest{Username: knownUsernames[i], Password: "definitely-wrong"})
		knownElapsed += time.Since(start)
	}
	for i := 0; i < trials; i++ {
		start := time.Now()
		_, _ = s.Login(ctx, backendrpc.LoginRequest{Username: fmt.Sprintf("timing-unknown-%d", i), Password: "definitely-wrong"})
		unknownElapsed += time.Since(start)
	}

	ratio := float64(unknownElapsed) / float64(knownElapsed)
	t.Logf("known-username elapsed=%v unknown-username elapsed=%v ratio=%.3f", knownElapsed, unknownElapsed, ratio)
	if ratio < 0.4 {
		t.Fatalf("Login: unknown-username attempts finished in %v vs %v for known-username attempts "+
			"(unknown is %.1fx faster) — this timing gap lets an attacker enumerate valid usernames "+
			"purely from response latency, even though the error message is identical", unknownElapsed, knownElapsed, 1/ratio)
	}
}

func codeOf(err error) codes.Code {
	return status.Code(err)
}

// TestAuthServerRequestPhoneVerificationGlobalCeilingCapsDistinctPhoneSpray
// proves RequestPhoneVerification cannot be used to run up an unbounded
// Twilio SMS bill by spraying many DIFFERENT real phone numbers. Each phone
// number gets its own phoneVerifyLimiter bucket and each fabricated
// DeviceLabel gets its own deviceVerifyLimiter bucket (see
// authServer.RequestPhoneVerification), so an attacker who targets a fresh
// phone number with a fresh device label on every call never reuses either
// bucket and, before this test's accompanying fix, could trigger real,
// money-costing SMS sends with no ceiling at all.
func TestAuthServerRequestPhoneVerificationGlobalCeilingCapsDistinctPhoneSpray(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)

	const attempts = 60
	exhausted := 0
	for i := 0; i < attempts; i++ {
		_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
			PhoneNumber: fmt.Sprintf("+1555987%04d", i),
			DeviceLabel: fmt.Sprintf("spray-device-%d", i),
		})
		if err != nil && codeOf(err) == codes.ResourceExhausted {
			exhausted++
			continue
		}
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if exhausted == 0 {
		t.Fatalf("RequestPhoneVerification: %d distinct real-looking phone numbers each got a real SMS send "+
			"(verify.sendCalls=%d) with a fresh DeviceLabel every time and none throttled — an attacker can "+
			"spray unlimited distinct phone numbers to run up an arbitrarily large Twilio bill, since "+
			"phoneVerifyLimiter/deviceVerifyLimiter are both keyed by caller-supplied values with no "+
			"non-bypassable global ceiling", attempts, verify.sendCalls)
	}
}

// TestAuthServerRegisterRejectsOverlongUsername proves Register caps username
// length server-side rather than trusting the wasm client's own validation
// (internal/app/authcredentials.go), which a caller can trivially bypass by
// calling this RPC directly with no client at all.
func TestAuthServerRegisterRejectsOverlongUsername(t *testing.T) {
	s := newTestAuthServer(t)
	huge := strings.Repeat("a", maxUsernameLength+1)
	_, err := s.Register(context.Background(), backendrpc.RegisterRequest{Username: huge, Password: "correct-horse-battery"})
	if err == nil {
		t.Fatal("Register: expected an error for a username past maxUsernameLength")
	} else if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("Register: expected InvalidArgument for an overlong username, got %v", err)
	}
}
