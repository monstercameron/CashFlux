// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Findings 5, 6 and 7 from the 2026-08-18 auth review, plus the OAuth returnTo
// hazard. These all concern what a credential change is supposed to accomplish:
// changing a password is the remediation for a compromise, so it has to actually
// evict whoever is already in, and it must not itself be a way in.

func newCredentialTestService(t *testing.T) (*authServer, *Store, Config) {
	t.Helper()
	store := openTestStore(t)
	cfg := withSessionKey(t, Config{AuthMode: "token", Token: "static-token"}, store)
	return newAuthService(store, cfg), store, cfg
}

// registerCam creates an account and returns its id plus its first session.
func registerCam(t *testing.T, svc *authServer) (userID string, pair backendrpc.TokenPairResponse) {
	t.Helper()
	out, err := svc.Register(context.Background(), backendrpc.RegisterRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "laptop",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return localUserID("cam"), out
}

// ---------------------------------------------------------------------------
// Finding 6 — changing an existing password needed no proof of the current one
// ---------------------------------------------------------------------------

// Holding a live session used to be enough to rewrite the account's username and
// password and mint a fresh recovery code. Every paired device was therefore a
// silent, complete account takeover, with the owner locked out and no
// notification anywhere.
func TestSetPasswordRequiresTheCurrentPassword(t *testing.T) {
	svc, _, _ := newCredentialTestService(t)
	userID, _ := registerCam(t, svc)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})

	_, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "cam", Password: "attacker-chosen-pw"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("change with no current password: err = %v, want InvalidArgument", err)
	}

	_, err = svc.SetPassword(ctx, backendrpc.SetPasswordRequest{
		Username: "cam", Password: "attacker-chosen-pw", CurrentPassword: "not-the-password",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("change with a wrong current password: err = %v, want Unauthenticated", err)
	}

	// The account is untouched: the original password still works.
	if _, err := svc.Login(context.Background(), backendrpc.LoginRequest{
		Username: "cam", Password: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("original password stopped working after two failed change attempts: %v", err)
	}
}

// The genuine change must still go through.
func TestSetPasswordWithTheCurrentPasswordSucceeds(t *testing.T) {
	svc, _, _ := newCredentialTestService(t)
	userID, _ := registerCam(t, svc)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})

	out, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{
		Username: "cam", Password: "a-brand-new-password", CurrentPassword: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("legitimate change: %v", err)
	}
	if strings.TrimSpace(out.RecoveryCode) == "" {
		t.Fatal("no replacement recovery code was issued")
	}
	if _, err := svc.Login(context.Background(), backendrpc.LoginRequest{
		Username: "cam", Password: "a-brand-new-password",
	}); err != nil {
		t.Fatalf("new password does not work: %v", err)
	}
	if _, err := svc.Login(context.Background(), backendrpc.LoginRequest{
		Username: "cam", Password: "correct-horse-battery",
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("old password still works: %v", err)
	}
}

// The bootstrap case is exempt by design: a freshly paired device is setting the
// account's FIRST password and has no prior secret to prove.
func TestFirstPasswordNeedsNoCurrentPassword(t *testing.T) {
	svc, store, _ := newCredentialTestService(t)
	mustUser(t, store, "device:fresh", "device")
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "device:fresh"})

	out, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "newdevice", Password: "first-password-here"})
	if err != nil {
		t.Fatalf("bootstrap SetPassword: %v", err)
	}
	if strings.TrimSpace(out.RecoveryCode) == "" {
		t.Fatal("bootstrap issued no recovery code")
	}
	// And having set one, a SECOND change now requires it.
	if _, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "newdevice", Password: "second-password"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("second change without the current password: err = %v, want InvalidArgument", err)
	}
}

// ---------------------------------------------------------------------------
// Finding 5 — SetPassword revoked nothing (ResetPassword already did)
// ---------------------------------------------------------------------------

// SetPassword left every existing refresh family alive for the rest of its
// 30-day life, so the attacker whose access prompted the change kept it. (The
// recovery path never had this bug — see the test below.)
func TestSetPasswordEvictsEveryOtherSession(t *testing.T) {
	svc, _, _ := newCredentialTestService(t)
	userID, owner := registerCam(t, svc)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})

	// A second device is signed in — think of it as the one that should not be.
	intruder, err := svc.Login(context.Background(), backendrpc.LoginRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "intruder",
	})
	if err != nil {
		t.Fatalf("second session: %v", err)
	}

	out, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{
		Username: "cam", Password: "a-brand-new-password", CurrentPassword: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}

	// Both pre-existing refresh tokens are dead.
	for name, refresh := range map[string]string{"intruder": intruder.RefreshToken, "owner's old": owner.RefreshToken} {
		if _, err := svc.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: refresh}); err == nil {
			t.Fatalf("the %s refresh token survived a password change", name)
		}
	}

	// And the caller is handed a working replacement, so protecting the account
	// does not sign out the person doing the protecting.
	if strings.TrimSpace(out.AccessToken) == "" || strings.TrimSpace(out.RefreshToken) == "" {
		t.Fatal("SetPassword revoked the caller's session without issuing a replacement")
	}
	if _, err := svc.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: out.RefreshToken}); err != nil {
		t.Fatalf("the replacement session does not work: %v", err)
	}
}

// Recovery already revoked every prior session (RotateLocalRecovery does it in
// the credential-swap transaction). Pinned here alongside the SetPassword case
// so the two halves of "a credential change evicts everyone else" cannot drift
// apart — only SetPassword was missing it.
func TestResetPasswordEvictsEveryOtherSession(t *testing.T) {
	svc, _, _ := newCredentialTestService(t)
	registerOut, err := svc.Register(context.Background(), backendrpc.RegisterRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "laptop",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	intruder, err := svc.Login(context.Background(), backendrpc.LoginRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "intruder",
	})
	if err != nil {
		t.Fatalf("intruder session: %v", err)
	}

	recovered, err := svc.ResetPassword(context.Background(), backendrpc.ResetPasswordRequest{
		Username: "cam", RecoveryCode: registerOut.RecoveryCode, NewPassword: "recovered-password",
	})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}

	if _, err := svc.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: intruder.RefreshToken}); err == nil {
		t.Fatal("account recovery left the intruder's session alive")
	}
	if _, err := svc.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: recovered.RefreshToken}); err != nil {
		t.Fatalf("recovery revoked the session it had just issued: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Finding 7 — the replacement recovery code was derived from a raw config field
// ---------------------------------------------------------------------------

// deterministicRecoveryCode read cfg.SessionKey directly rather than the resolved
// signing secret. That field is legitimately empty whenever the operator set a
// MasterKey instead, or the server generated its own key — and HMAC under an
// empty key makes the new recovery code a pure function of the user id and the
// idempotency key.
func TestReplacementRecoveryCodeIsNotDerivedFromAnEmptyKey(t *testing.T) {
	store := openTestStore(t)
	// A MasterKey deployment: cfg.SessionKey stays empty on purpose.
	cfg := withSessionKey(t, Config{MasterKey: "0123456789abcdef0123456789abcdef", Token: "t"}, store)
	if strings.TrimSpace(cfg.SessionKey) != "" {
		t.Fatalf("precondition: SessionKey should be empty here, got %q", cfg.SessionKey)
	}
	svc := newAuthService(store, cfg)

	out, err := svc.Register(context.Background(), backendrpc.RegisterRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "laptop",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	const idemKey = "known-idempotency-key"
	reset, err := svc.ResetPassword(context.Background(), backendrpc.ResetPasswordRequest{
		Username: "cam", RecoveryCode: out.RecoveryCode, NewPassword: "recovered-password",
		IdempotencyKey: idemKey,
	})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	// What an attacker would compute if the key were empty.
	guessed := deterministicRecoveryCode("", localUserID("cam"), idemKey)
	if reset.RecoveryCode == guessed {
		t.Fatal("the replacement recovery code is derivable under an empty HMAC key")
	}
	// It must still be derived from the REAL secret, so a retry replays it.
	if want := deterministicRecoveryCode(string(sessionSigningSecret(cfg)), localUserID("cam"), idemKey); reset.RecoveryCode != want {
		t.Fatal("the replacement code is not derived from the resolved signing secret")
	}
}

// ---------------------------------------------------------------------------
// OAuth returnTo — a wildcard app origin must not authorize handing out a token
// ---------------------------------------------------------------------------

// writeOAuthCallbackHTML posts the ACCESS TOKEN to the returnTo origin, and
// returnTo is an unauthenticated query parameter. allowedOrigin honours a "*"
// app origin, so under a wildcard any site could have been named as the
// destination. "*" is a statement about who may call the API, never about who
// may be handed somebody's session.
func TestOAuthReturnToRejectsWildcardOrigin(t *testing.T) {
	for _, tc := range []struct {
		name      string
		appOrigin string
		returnTo  string
		want      string
	}{
		{"wildcard never authorizes a destination", "*", "https://evil.example/steal", ""},
		{"wildcard rejects even the app's own origin", "*", "https://budget.example/app", ""},
		{"a configured origin is honoured", "https://budget.example", "https://budget.example/app", "https://budget.example/app"},
		{"a foreign origin is refused", "https://budget.example", "https://evil.example/steal", ""},
		{"a relative target is refused", "https://budget.example", "/app", ""},
		{"no returnTo at all", "https://budget.example", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := "/v1/auth/google/start"
			if tc.returnTo != "" {
				target += "?returnTo=" + tc.returnTo
			}
			r := httptest.NewRequest("GET", target, nil)
			if got := validatedOAuthReturnTo(r, Config{AppOrigin: tc.appOrigin}); got != tc.want {
				t.Fatalf("validatedOAuthReturnTo = %q, want %q", got, tc.want)
			}
		})
	}
}

// Config validation is the other half of that defence and must keep rejecting a
// wildcard app origin outright.
func TestWildcardAppOriginIsRejectedByConfigValidation(t *testing.T) {
	cfg := Config{Addr: ":8080", DataDir: t.TempDir(), AppOrigin: "*", AuthMode: "token", Token: "t"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Config.Validate accepted a wildcard app origin")
	}
}

// A sanity check that the whole credential-change path still leaves the account
// usable end to end, rather than only that each guard fires.
func TestCredentialChangeRoundTrip(t *testing.T) {
	svc, store, _ := newCredentialTestService(t)
	userID, _ := registerCam(t, svc)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})

	if _, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{
		Username: "cam", Password: "second-password-ok", CurrentPassword: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("first change: %v", err)
	}
	if _, err := svc.SetPassword(ctx, backendrpc.SetPasswordRequest{
		Username: "cam", Password: "third-password-ok", CurrentPassword: "second-password-ok",
	}); err != nil {
		t.Fatalf("second change: %v", err)
	}
	final, err := svc.Login(context.Background(), backendrpc.LoginRequest{Username: "cam", Password: "third-password-ok"})
	if err != nil {
		t.Fatalf("login after two changes: %v", err)
	}
	// The session works against a real service call.
	sync := NewSyncService(store)
	now := time.Now().UTC()
	if _, err := sync.PutWorkspace(ContextWithAuthUser(context.Background(), AuthUser{ID: userID}),
		Workspace{ID: "ws1", Name: "W"}, now, false, now); err != nil {
		t.Fatalf("sync after credential changes: %v", err)
	}
	if strings.TrimSpace(final.AccessToken) == "" {
		t.Fatal("final login issued no access token")
	}
}
