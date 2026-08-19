// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This file covers the auth findings from the 2026-08-18 review. Each test names
// the behaviour that was wrong, so a regression reads as a sentence rather than
// as an assertion failure on a line number.

// ---------------------------------------------------------------------------
// Finding 1 — a deleted account could be resurrected by a still-valid token
// ---------------------------------------------------------------------------

// An access token is a stateless JWT with a 15-minute life and no revocation
// check, so a browser open at the moment of deletion keeps pushing afterwards.
// ensureUserRow used to answer that by recreating the account — with the default
// role and, because suspension is a column on that same row, unsuspended.
func TestDeletedAccountIsNotRecreatedBySync(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u-erase"})

	mustUser(t, store, "u-erase", "local")
	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "ws-money", Name: "Money"}, now, false, now); err != nil {
		t.Fatalf("initial push: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{UserID: "u-erase", WorkspaceID: "ws-money", Dataset: []byte(`{"secret":"salary"}`), Version: 1, UpdatedAt: now}, 0, 5); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	if deleted, err := store.DeleteAccount("u-erase"); err != nil || !deleted {
		t.Fatalf("delete account: deleted=%v err=%v", deleted, err)
	}

	// The tab is still open and its queue flushes exactly as it always does.
	later := now.Add(time.Minute)
	_, err := svc.PutWorkspace(ctx, Workspace{ID: "ws-money", Name: "Money"}, later, false, later)
	if err == nil {
		t.Fatal("a push from a deleted account was accepted — the account has been resurrected")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("push error = %v, want Unauthenticated", err)
	}
	if _, found, _ := store.GetUserByID("u-erase"); found {
		t.Fatal("the deleted account exists again")
	}
	if _, ok, _ := store.GetSnapshot("u-erase", "ws-money"); ok {
		t.Fatal("the deleted snapshot came back")
	}
}

// Suspension lives on the users row, so deleting the row cleared it. That turned
// self-service deletion into a suspension bypass: delete, then let the
// still-valid access token rebuild the account without the suspension.
func TestSuspendedAccountCannotEraseItsWaySuspensionFree(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u-susp"})

	mustUser(t, store, "u-susp", "local")
	if err := store.SetUserSuspended("u-susp", true, now); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "ws1", Name: "W"}, now, false, now); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("suspended push error = %v, want PermissionDenied", err)
	}

	// The user's own bearer token still works against DELETE /v1/account.
	if _, err := store.DeleteAccount("u-susp"); err != nil {
		t.Fatalf("self delete: %v", err)
	}
	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "ws1", Name: "W"}, now, false, now); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("post-delete push error = %v, want Unauthenticated", err)
	}
	if _, found, _ := store.GetUserByID("u-susp"); found {
		t.Fatal("the suspended account rebuilt itself by deleting itself")
	}
}

// The tombstone must also stop the credential authenticating at all, not merely
// stop it writing.
func TestSessionTokenForDeletedAccountStopsAuthenticating(t *testing.T) {
	store := openTestStore(t)
	cfg := withSessionKey(t, Config{AuthMode: "token", Token: "static"}, store)
	mustUser(t, store, "u-gone", "local")

	token, err := issueSessionToken(cfg, "u-gone", "access", time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if user, ok := authUserForToken(token, cfg, store); !ok || user.ID != "u-gone" {
		t.Fatalf("token should authenticate before deletion: ok=%v id=%q", ok, user.ID)
	}
	if _, err := store.DeleteAccount("u-gone"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := authUserForToken(token, cfg, store); ok {
		t.Fatal("a session token for a deleted account still authenticates")
	}
}

// A tombstone must not become a permanent ban on an id. Local ids are derived
// from the username, so re-registering a freed name lands on the same id, and a
// person signing up again is not the stale credential the tombstone guards.
func TestDeletedUsernameCanRegisterAgain(t *testing.T) {
	store := openTestStore(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-horse-battery"), bcrypt.MinCost)
	now := time.Now().UTC()

	first, err := store.CreateLocalUser("marcus", string(hash), string(hash), now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DeleteAccount(first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted, err := store.AccountDeleted(first.ID)
	if err != nil || !deleted {
		t.Fatalf("expected a tombstone: deleted=%v err=%v", deleted, err)
	}
	again, err := store.CreateLocalUser("marcus", string(hash), string(hash), now)
	if err != nil {
		t.Fatalf("re-register the freed username: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("expected the same derived id, got %q then %q", first.ID, again.ID)
	}
	if deleted, _ := store.AccountDeleted(again.ID); deleted {
		t.Fatal("the tombstone survived a deliberate re-registration")
	}
	// And the rebuilt account must actually work.
	svc := NewSyncService(store)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: again.ID})
	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "ws-new", Name: "New"}, now, false, now); err != nil {
		t.Fatalf("re-registered account cannot sync: %v", err)
	}
}

// An id nobody has ever deleted must still materialize on demand — that is
// ensureUserRow's actual job, and token-mode identities depend on it.
func TestTokenModeIdentityStillMaterializes(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "token:abc123"})

	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "ws1", Name: "W"}, now, false, now); err != nil {
		t.Fatalf("first push from a never-seen token identity: %v", err)
	}
	if _, found, _ := store.GetUserByID("token:abc123"); !found {
		t.Fatal("ensureUserRow no longer materializes a fresh token identity")
	}
}

// ---------------------------------------------------------------------------
// Finding 2 — the session signing secret must never be a client-held credential
// ---------------------------------------------------------------------------

// The static bearer token is handed to every syncing client. When it was also
// the JWT signing key, any client could mint a session for any `sub` on the
// server — the owner account included.
func TestSessionSecretIsNeverTheClientToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"plain token", Config{AuthMode: "token", Token: "shared-sync-token"}},
		{"token hash only", Config{AuthMode: "token", TokenSHA256: strings.Repeat("ab", 32)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if secret := sessionSigningSecret(tc.cfg); len(secret) != 0 {
				t.Fatalf("signing secret = %q, want none — a client-held credential must not sign sessions", secret)
			}
			// And with no secret, no session can be minted at all: fail closed.
			if _, err := issueSessionToken(tc.cfg, "device:owner", "access", time.Hour, time.Now().UTC()); err == nil {
				t.Fatal("a session was issued with no server-side signing secret")
			}
		})
	}
}

// A holder of the static token must not be able to forge a session for a named
// account, even once the server has resolved its own signing key.
func TestStaticTokenCannotForgeAnotherIdentity(t *testing.T) {
	store := openTestStore(t)
	const clientToken = "shared-sync-token"
	cfg := withSessionKey(t, Config{AuthMode: "token", Token: clientToken}, store)

	// What an attacker holding the client token can build: the same JWT shape,
	// signed with the only secret they have.
	forgery := Config{SessionKey: clientToken}
	forged, err := issueSessionToken(forgery, "device:owner", "access", time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("build forgery: %v", err)
	}
	if user, ok := authUserForToken(forged, cfg, store); ok {
		t.Fatalf("a token signed with the client's own bearer token authenticated as %q", user.ID)
	}
	// The token itself still works — as its own synthetic identity, never as
	// somebody else's account.
	user, ok := authUserForToken(clientToken, cfg, store)
	if !ok || !strings.HasPrefix(user.ID, "token:") {
		t.Fatalf("static token identity = %q ok=%v, want a token: identity", user.ID, ok)
	}
}

// A forged REFRESH token is the more valuable forgery — it buys 30 days rather
// than 15 minutes — so it gets its own check.
func TestStaticTokenCannotForgeARefreshToken(t *testing.T) {
	store := openTestStore(t)
	const clientToken = "shared-sync-token"
	cfg := withSessionKey(t, Config{AuthMode: "token", Token: clientToken}, store)

	forged, err := issueSessionTokenWithClaims(Config{SessionKey: clientToken}, sessionClaims{
		Sub: "device:owner", Type: "refresh", Exp: time.Now().Add(720 * time.Hour).Unix(),
		JTI: "forged-jti", Family: "forged-family",
	})
	if err != nil {
		t.Fatalf("build forgery: %v", err)
	}
	if _, ok := verifySessionToken(cfg, forged, "refresh", time.Now().UTC()); ok {
		t.Fatal("a refresh token signed with the client bearer token verified")
	}
}

// The generated secret has to survive a restart, or every deploy signs everyone
// out, and two processes on one store must agree on it.
func TestResolvedSessionKeyIsStableAndPrivate(t *testing.T) {
	store := openTestStore(t)
	base := Config{AuthMode: "token", Token: "shared-sync-token"}

	first := withSessionKey(t, base, store)
	second := withSessionKey(t, base, store)
	if first.SessionKey != second.SessionKey {
		t.Fatal("the resolved session key changed between resolutions — sessions would not survive a restart")
	}
	if strings.TrimSpace(first.SessionKey) == "" {
		t.Fatal("no session key was resolved")
	}
	if first.SessionKey == base.Token || first.SessionKey == base.TokenSHA256 {
		t.Fatal("the resolved key is the client-held credential")
	}
	// A token minted under it verifies.
	token, err := issueSessionToken(first, "u1", "access", time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if sub, ok := verifySessionToken(second, token, "access", time.Now().UTC()); !ok || sub != "u1" {
		t.Fatalf("verify across resolutions: sub=%q ok=%v", sub, ok)
	}
}

// An operator-configured key always wins and is never replaced by a generated one.
func TestConfiguredSessionKeysAreNeverOverwritten(t *testing.T) {
	store := openTestStore(t)
	for _, tc := range []struct {
		name string
		want string
		cfg  Config
	}{
		{"explicit session key", "operator-chosen", Config{SessionKey: "operator-chosen", Token: "client-token"}},
		{"master key fallback", "", Config{MasterKey: "0123456789abcdef0123456789abcdef", Token: "client-token"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := withSessionKey(t, tc.cfg, store)
			if tc.want != "" && got.SessionKey != tc.want {
				t.Fatalf("SessionKey = %q, want %q", got.SessionKey, tc.want)
			}
			if tc.want == "" && got.SessionKey != "" {
				t.Fatalf("a MasterKey deployment had a session key generated over it: %q", got.SessionKey)
			}
			if string(sessionSigningSecret(got)) == tc.cfg.Token {
				t.Fatal("signing fell back to the client token")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Finding 3 — Login had no server-wide rate limit
// ---------------------------------------------------------------------------

// loginLimiter is keyed by username, which bounds guessing one account and
// nothing else: a spray across many usernames never trips it. Each attempt also
// costs a full bcrypt comparison, so it was a CPU-exhaustion door too.
func TestLoginSprayAcrossUsernamesIsRateLimited(t *testing.T) {
	store := openTestStore(t)
	svc := newAuthService(store, withSessionKey(t, Config{Token: "t"}, store))
	ctx := context.Background()

	refused := 0
	attempts := loginGlobalLimitPerMinute + 25
	for i := 0; i < attempts; i++ {
		_, err := svc.Login(ctx, backendrpc.LoginRequest{
			Username: fmt.Sprintf("victim%d", i), // a different bucket every time
			Password: "one-common-password",
		})
		if status.Code(err) == codes.ResourceExhausted {
			refused++
		}
	}
	if refused == 0 {
		t.Fatalf("%d login attempts across distinct usernames, none rate limited", attempts)
	}
	if want := attempts - loginGlobalLimitPerMinute; refused != want {
		t.Fatalf("refused %d of %d attempts, want %d once the global budget is spent", refused, attempts, want)
	}
}

// The per-username limit must still bound guessing against a single account.
func TestLoginPerUsernameLimitStillApplies(t *testing.T) {
	store := openTestStore(t)
	svc := newAuthService(store, withSessionKey(t, Config{Token: "t"}, store))
	ctx := context.Background()

	refused := 0
	for i := 0; i < loginLimitPerMinute+5; i++ {
		_, err := svc.Login(ctx, backendrpc.LoginRequest{Username: "one-victim", Password: "guess"})
		if status.Code(err) == codes.ResourceExhausted {
			refused++
		}
	}
	if refused != 5 {
		t.Fatalf("refused %d attempts against one username, want 5 past the %d cap", refused, loginLimitPerMinute)
	}
}

// A real person's sign-in must not be collateral damage.
func TestOrdinaryLoginIsNotRateLimited(t *testing.T) {
	store := openTestStore(t)
	cfg := withSessionKey(t, Config{Token: "t"}, store)
	svc := newAuthService(store, cfg)
	ctx := context.Background()

	if _, err := svc.Register(ctx, backendrpc.RegisterRequest{
		Username: "cam", Password: "correct-horse-battery", DeviceLabel: "laptop",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := svc.Login(ctx, backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
			t.Fatalf("ordinary login %d: %v", i, err)
		}
	}
}

func mustUser(t *testing.T, store *Store, id, provider string) {
	t.Helper()
	if err := store.UpsertUser(User{ID: id, Provider: provider, Subject: id, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}
