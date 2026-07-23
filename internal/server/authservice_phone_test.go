// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeVerifyClient is a table-driven-friendly, in-memory VerifyClient stand-in
// for internal/twilio.VerifyClient (TODOS.md C420 test double) — no real
// Twilio calls happen in these tests.
type fakeVerifyClient struct {
	mu         sync.Mutex // guards the fields below for the concurrent-verification test
	sendErr    error
	sentCodes  map[string]string // phone -> code "sent"
	sendCalls  int
	checkCalls int
	checkErr   error
}

func newFakeVerifyClient() *fakeVerifyClient {
	return &fakeVerifyClient{sentCodes: map[string]string{}}
}

func (f *fakeVerifyClient) SendCode(_ context.Context, phone string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendCalls++
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sentCodes[phone] = "123456"
	return nil
}

func (f *fakeVerifyClient) CheckCode(_ context.Context, phone, code string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkCalls++
	if f.checkErr != nil {
		return false, f.checkErr
	}
	return f.sentCodes[phone] != "" && f.sentCodes[phone] == code, nil
}

func newPhoneTestAuthServer(t *testing.T, verify *fakeVerifyClient) *authServer {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "cashflux-server.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := Config{SessionKey: "test-session-key-1234567890"}
	s := newAuthService(store, cfg)
	s.verify = verify
	return s
}

// newGatedPhoneTestAuthServer is newPhoneTestAuthServer with Config.SetupCode
// configured, for the setup-code enrollment-gate tests below.
func newGatedPhoneTestAuthServer(t *testing.T, verify *fakeVerifyClient, setupCode string) *authServer {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "cashflux-server.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := Config{SessionKey: "test-session-key-1234567890", SetupCode: setupCode}
	s := newAuthService(store, cfg)
	s.verify = verify
	return s
}

// TestRequestPhoneVerificationRejectsMissingSetupCode proves a deployment
// with Config.SetupCode configured refuses to send an SMS for a brand-new
// phone number when no (or the wrong) setup code is presented — the
// fail-fast half of the gate, so a wrong guess never costs a Twilio send.
func TestRequestPhoneVerificationRejectsMissingSetupCode(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newGatedPhoneTestAuthServer(t, verify, "invite-123")
	_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551239001",
	})
	assertGRPCCode(t, err, codes.PermissionDenied)
	if verify.sendCalls != 0 {
		t.Fatalf("SendCode calls = %d, want 0 (must not send SMS without a valid setup code)", verify.sendCalls)
	}

	_, err = s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551239001", SetupCode: "wrong-code",
	})
	assertGRPCCode(t, err, codes.PermissionDenied)
	if verify.sendCalls != 0 {
		t.Fatalf("SendCode calls = %d, want 0 (wrong setup code must not send SMS)", verify.sendCalls)
	}
}

// TestVerifyPhoneCodeSetupCodeGateEndToEnd proves the full gated-enrollment
// contract: a correct setup code lets a new account through and consumes the
// code (a second phone number can no longer redeem the same code), while a
// returning phone number (already verified once) never needs the code again.
func TestVerifyPhoneCodeSetupCodeGateEndToEnd(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newGatedPhoneTestAuthServer(t, verify, "invite-123")
	phone := "+15551239002"

	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: phone, SetupCode: "invite-123",
	}); err != nil {
		t.Fatalf("RequestPhoneVerification with valid setup code: %v", err)
	}

	// A correct SMS code but no setup code on VerifyPhoneCode must still fail —
	// RequestPhoneVerification only fail-fast-checks; VerifyPhoneCode is the
	// authoritative gate.
	if _, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("VerifyPhoneCode without setup code: err = %v, want PermissionDenied", err)
	}

	resp, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456", SetupCode: "invite-123",
	})
	if err != nil {
		t.Fatalf("VerifyPhoneCode with valid setup code: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("resp = %+v, want a minted access token", resp)
	}

	// The code is single-use: a second, different phone number can't redeem it.
	otherPhone := "+15551239003"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: otherPhone, SetupCode: "invite-123",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RequestPhoneVerification for a second phone with a spent code: err = %v, want PermissionDenied", err)
	}

	// The now-verified original phone number can sign in again on another
	// device (e.g. RequestPhoneVerification/VerifyPhoneCode from a second
	// device) with no setup code at all — it's a returning account, not a new
	// invite.
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: phone, DeviceLabel: "second-device",
	}); err != nil {
		t.Fatalf("RequestPhoneVerification for a returning phone number: %v", err)
	}
	if _, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456", DeviceLabel: "second-device",
	}); err != nil {
		t.Fatalf("VerifyPhoneCode for a returning phone number: %v", err)
	}
}

func TestRequestPhoneVerificationHappyPath(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	resp, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551230001",
		DeviceLabel: "unit-test-device",
	})
	if err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	if !resp.Sent {
		t.Fatalf("resp.Sent = false, want true")
	}
	if verify.sendCalls != 1 {
		t.Fatalf("SendCode calls = %d, want 1", verify.sendCalls)
	}
}

func TestRequestPhoneVerificationInvalidPhone(t *testing.T) {
	s := newPhoneTestAuthServer(t, newFakeVerifyClient())
	_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: "5551230002"})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestRequestPhoneVerificationRateLimitedPerPhone(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230003"
	for i := 0; i < phoneVerifyLimitPerMinute; i++ {
		// Distinct device labels per call so only the phone-scoped limiter is
		// exercised, isolating it from the device-scoped one.
		req := backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone, DeviceLabel: deviceLabelForIndex(i)}
		if _, err := s.RequestPhoneVerification(context.Background(), req); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: phone, DeviceLabel: deviceLabelForIndex(phoneVerifyLimitPerMinute),
	})
	assertGRPCCode(t, err, codes.ResourceExhausted)
}

func TestRequestPhoneVerificationRateLimitedPerDevice(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	device := "shared-device"
	for i := 0; i < deviceVerifyLimitPerMinute; i++ {
		// Distinct phone numbers per call so only the device-scoped limiter is
		// exercised, isolating it from the phone-scoped one.
		req := backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phoneForIndex(i), DeviceLabel: device}
		if _, err := s.RequestPhoneVerification(context.Background(), req); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: phoneForIndex(deviceVerifyLimitPerMinute), DeviceLabel: device,
	})
	assertGRPCCode(t, err, codes.ResourceExhausted)
}

func TestRequestPhoneVerificationIdempotentRetryDoesNotResend(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	req := backendrpc.RequestPhoneVerificationRequest{PhoneNumber: "+15551230010", DeviceLabel: "retry-device"}
	if _, err := s.RequestPhoneVerification(context.Background(), req); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := s.RequestPhoneVerification(context.Background(), req); err != nil {
		t.Fatalf("retried call: %v", err)
	}
	if verify.sendCalls != 1 {
		t.Fatalf("SendCode calls = %d, want 1 (retry within the dedupe window must not resend)", verify.sendCalls)
	}
}

func TestVerifyPhoneCodeHappyPath(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230020"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	resp, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456", DeviceLabel: "phone-x",
	})
	if err != nil {
		t.Fatalf("VerifyPhoneCode: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("resp = %+v, want non-empty tokens", resp)
	}
	user, ok, err := s.store.GetUserByID("phone:" + phone)
	if err != nil || !ok {
		t.Fatalf("GetUserByID(phone:%s) = %v/%v, want a created user", phone, ok, err)
	}
	if user.Provider != "phone" || user.Subject != phone {
		t.Fatalf("user = %+v, want provider=phone subject=%s", user, phone)
	}
}

func TestVerifyPhoneCodeWrongCode(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230021"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	_, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "000000"})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestVerifyPhoneCodeExpiredOrUnknownVerification(t *testing.T) {
	// No RequestPhoneVerification call precedes this: the fake client has no
	// code on file for the phone, mirroring Twilio's "expired/unknown
	// verification" outcome (CheckCode returns false, not an error).
	s := newPhoneTestAuthServer(t, newFakeVerifyClient())
	_, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{PhoneNumber: "+15551230022", Code: "123456"})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestVerifyPhoneCodeIdempotentRetryReturnsSameTokenPair(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230023"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	req := backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456", DeviceLabel: "retry-device", IdempotencyKey: "idem-key-1",
	}
	first, err := s.VerifyPhoneCode(context.Background(), req)
	if err != nil {
		t.Fatalf("first VerifyPhoneCode: %v", err)
	}
	second, err := s.VerifyPhoneCode(context.Background(), req)
	if err != nil {
		t.Fatalf("retried VerifyPhoneCode: %v", err)
	}
	if second != first {
		t.Fatalf("retried token pair = %+v, want identical to first %+v", second, first)
	}
	if verify.checkCalls != 1 {
		t.Fatalf("CheckCode calls = %d, want 1 (idempotent retry must not re-check with Twilio)", verify.checkCalls)
	}
}

// TestVerifyPhoneCodeRateLimited proves repeated wrong-code guesses against
// one phone number get throttled — otherwise a six-digit SMS code would be
// brute-forceable at our layer regardless of what Twilio itself enforces.
func TestVerifyPhoneCodeRateLimited(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230030"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	for i := 0; i < checkCodeLimitPerMinute; i++ {
		req := backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "000000", DeviceLabel: deviceLabelForIndex(i)}
		if _, err := s.VerifyPhoneCode(context.Background(), req); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("call %d: err = %v, want Unauthenticated below the limit", i, err)
		}
	}
	req := backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "000000", DeviceLabel: deviceLabelForIndex(checkCodeLimitPerMinute)}
	_, err := s.VerifyPhoneCode(context.Background(), req)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("call over the limit: err = %v, want ResourceExhausted", err)
	}
}

// TestConcurrentPhoneVerificationResolvesToSameAccount runs two concurrent
// VerifyPhoneCode calls for the SAME phone number (two devices racing to
// finish enrollment for one number, e.g. a user who tapped the SMS link
// twice) and proves exactly one account ever exists for that number
// afterward — both calls resolve to the SAME deterministic user id
// ("phone:"+phone, see phoneUserID) rather than two distinct accounts
// silently splitting one phone number's data across two ids. This is the
// server's actual defense against phone-number reuse (via the
// users(provider, subject) UNIQUE constraint from serverSchemaV1 and
// UpsertUser's ON CONFLICT upsert) — not the phone_number column/index added
// in migrateTo8, which nothing ever writes to (see
// TestPhoneNumberColumnIsNeverPopulated in store_test.go).
func TestConcurrentPhoneVerificationResolvesToSameAccount(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15559990002"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}

	const n = 2
	var wg sync.WaitGroup
	results := make([]backendrpc.TokenPairResponse, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
				PhoneNumber: phone, Code: "123456", DeviceLabel: deviceLabelForIndex(i),
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("VerifyPhoneCode goroutine %d: %v", i, err)
		}
	}
	for i, r := range results {
		if r.AccessToken == "" || r.RefreshToken == "" {
			t.Fatalf("goroutine %d: empty token pair %+v", i, r)
		}
	}

	var accountCount int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM users WHERE provider = 'phone' AND subject = ?`, phone).Scan(&accountCount); err != nil {
		t.Fatalf("count phone accounts: %v", err)
	}
	if accountCount != 1 {
		t.Fatalf("phone accounts for %s = %d, want exactly 1 — concurrent verification created duplicate accounts for the same number", phone, accountCount)
	}
}

func TestVerifyPhoneCodeIdempotencyKeyReusedForDifferentRequestFails(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15551230024"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	first := backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "123456", IdempotencyKey: "shared-key"}
	if _, err := s.VerifyPhoneCode(context.Background(), first); err != nil {
		t.Fatalf("first VerifyPhoneCode: %v", err)
	}
	second := backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "123456", DeviceLabel: "a-different-device", IdempotencyKey: "shared-key"}
	_, err := s.VerifyPhoneCode(context.Background(), second)
	assertGRPCCode(t, err, codes.InvalidArgument)
}

// deviceLabelForIndex/phoneForIndex generate distinct-but-deterministic labels/
// phone numbers for the rate-limit isolation tests above.
func deviceLabelForIndex(i int) string { return "device-" + itoa(i) }
func phoneForIndex(i int) string       { return "+1555999" + padThree(i) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

func padThree(i int) string {
	s := itoa(i)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func assertGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("err = nil, want gRPC code %s", want)
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != want {
		t.Fatalf("err = %v, want gRPC code %s", err, want)
	}
}

// TestEnrollmentAcceptsAdminMintedInviteCode proves an admin-minted invite
// code (pkg/embed.Admin.MintInviteCode) works as an alternative to the fixed
// Config.SetupCode for gated enrollment — the whole point of adding it.
func TestEnrollmentAcceptsAdminMintedInviteCode(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newGatedPhoneTestAuthServer(t, verify, "static-fallback-code")
	inviteCode, _, err := s.store.MintInviteCode(time.Now().UTC())
	if err != nil {
		t.Fatalf("MintInviteCode: %v", err)
	}
	phone := "+15551239010"

	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: phone, SetupCode: inviteCode,
	}); err != nil {
		t.Fatalf("RequestPhoneVerification with a valid invite code: %v", err)
	}
	resp, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{
		PhoneNumber: phone, Code: "123456", SetupCode: inviteCode,
	})
	if err != nil {
		t.Fatalf("VerifyPhoneCode with a valid invite code: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("resp = %+v, want a minted access token", resp)
	}

	// Single-use: a second phone number can't redeem the same invite code.
	otherPhone := "+15551239011"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: otherPhone, SetupCode: inviteCode,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RequestPhoneVerification for a second phone with a spent invite code: err = %v, want PermissionDenied", err)
	}

	// The static fallback code set in cfg.SetupCode still works independently
	// of the invite-code mechanism — the two sources coexist.
	thirdPhone := "+15551239012"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: thirdPhone, SetupCode: "static-fallback-code",
	}); err != nil {
		t.Fatalf("RequestPhoneVerification with the static setup code: %v", err)
	}
}

// TestEnrollmentRejectsExpiredInviteCode proves an expired invite code is
// rejected even though it was validly minted and never consumed.
func TestEnrollmentRejectsExpiredInviteCode(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newGatedPhoneTestAuthServer(t, verify, "static-fallback-code")
	mintedAt := time.Now().UTC().Add(-InviteCodeTTL - time.Minute)
	inviteCode, _, err := s.store.MintInviteCode(mintedAt)
	if err != nil {
		t.Fatalf("MintInviteCode: %v", err)
	}
	_, err = s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551239013", SetupCode: inviteCode,
	})
	assertGRPCCode(t, err, codes.PermissionDenied)
}

// TestEnrollmentRejectsUnmintedCode proves an arbitrary guessed string that
// was never minted as an invite code (and doesn't match the static setup
// code) is rejected — the exact bug the setup-code gate's first draft had.
func TestEnrollmentRejectsUnmintedCode(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newGatedPhoneTestAuthServer(t, verify, "static-fallback-code")
	_, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551239014", SetupCode: "999999",
	})
	assertGRPCCode(t, err, codes.PermissionDenied)
}
