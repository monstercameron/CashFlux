// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testOAuthVerifier = "abcdefghijklmnopqrstuvwxyz0123456789-._~ABCDEFG"
const testOAuthReturnTo = "https://app.example/settings"

func issueTestOAuthCode(t *testing.T, store *Store, now time.Time) string {
	t.Helper()
	if err := store.UpsertUser(User{ID: "github:code-user", Provider: "github", Subject: "code-user", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	code, err := store.issueOAuthSessionCode("github:code-user", testOAuthReturnTo, pkceChallenge(testOAuthVerifier), now)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestOAuthSessionCodeBindingsAndExpiry(t *testing.T) {
	for _, tc := range []struct {
		name, verifier, returnTo string
		age                      time.Duration
		want                     bool
	}{
		{"correct verifier", testOAuthVerifier, testOAuthReturnTo, 0, true},
		{"just before expiry", testOAuthVerifier, testOAuthReturnTo, 60*time.Second - time.Nanosecond, true},
		{"exact expiry", testOAuthVerifier, testOAuthReturnTo, 60 * time.Second, false},
		{"expired", testOAuthVerifier, testOAuthReturnTo, 61 * time.Second, false},
		{"PKCE mismatch", strings.Repeat("z", 43), testOAuthReturnTo, 0, false},
		{"plain challenge as verifier", pkceChallenge(testOAuthVerifier), testOAuthReturnTo, 0, false},
		{"short verifier", "short", testOAuthReturnTo, 0, false},
		{"invalid verifier characters", strings.Repeat("+", 43), testOAuthReturnTo, 0, false},
		{"redirect mismatch", testOAuthVerifier, "https://app.example/other", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := openTestStore(t)
			now := time.Now().UTC()
			code := issueTestOAuthCode(t, store, now)
			cfg := Config{SessionKey: "oauth-code-session-secret"}
			pair, user, ok, err := store.redeemOAuthSessionCode(cfg, code, tc.verifier, tc.returnTo, now.Add(tc.age))
			if err != nil || ok != tc.want {
				t.Fatalf("redeem = %v, %v; want %v", ok, err, tc.want)
			}
			var sessions int
			if err := store.db.QueryRow(`SELECT count(*) FROM refresh_tokens`).Scan(&sessions); err != nil {
				t.Fatal(err)
			}
			if !ok {
				if sessions != 0 || pair.RefreshToken != "" {
					t.Fatal("failed redemption issued a session")
				}
				if tc.age < oauthSessionCodeTTL {
					_, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now)
					if err != nil || !ok {
						t.Fatalf("mismatch consumed the code: %v, %v", ok, err)
					}
				}
				return
			}
			if sessions != 1 || user != "github:code-user" || pair.AccessToken == "" || pair.RefreshToken == "" || pair.DeviceID == "" {
				t.Fatal("account-bound session pair missing")
			}
			claims, ok := verifySessionClaims(cfg, pair.RefreshToken, "refresh", now)
			if !ok || claims.Sub != user || claims.Family != pair.DeviceID {
				t.Fatal("refresh token not bound to code account")
			}
			stored, ok, err := store.GetRefreshSession(claims.JTI)
			if err != nil || !ok || stored.TokenHash != sessionTokenHash(pair.RefreshToken) {
				t.Fatal("session pair not durable")
			}
			if _, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now.Add(time.Second)); err != nil || ok {
				t.Fatalf("code replay = %v, %v", ok, err)
			}
		})
	}
}

func TestOAuthSessionCodeConcurrentRedemption(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	code := issueTestOAuthCode(t, store, now)
	cfg := Config{SessionKey: "oauth-code-session-secret"}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, 16)
	for range 16 {
		wg.Go(func() {
			<-start
			_, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now)
			if err != nil {
				t.Errorf("concurrent redeem: %v", err)
			}
			results <- ok
		})
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for ok := range results {
		if ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful redemptions = %d, want 1", winners)
	}
}

func TestOAuthSessionCodeDurableAndAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codes.sqlite")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	code := issueTestOAuthCode(t, store, now)
	var hash string
	if err := store.db.QueryRow(`SELECT code_hash FROM oauth_session_codes`).Scan(&hash); err != nil || hash != sessionTokenHash(code) || hash == code {
		t.Fatalf("code not stored as a hash: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	// Force a failure after code consumption. The transaction must put it back.
	if _, err := store.db.Exec(`CREATE TRIGGER reject_code_session BEFORE INSERT ON refresh_tokens BEGIN SELECT RAISE(ABORT, 'test storage failure'); END`); err != nil {
		t.Fatal(err)
	}
	cfg := Config{SessionKey: "oauth-code-session-secret"}
	if _, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now); err == nil || ok {
		t.Fatal("failed session persistence was acknowledged")
	}
	if _, err := store.db.Exec(`DROP TRIGGER reject_code_session`); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := store.redeemOAuthSessionCode(Config{}, code, testOAuthVerifier, testOAuthReturnTo, now); err == nil || ok {
		t.Fatal("failed signing was acknowledged")
	}
	if _, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now); err != nil || !ok {
		t.Fatalf("failed exchange consumed code: %v, %v", ok, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, _, ok, err := store.redeemOAuthSessionCode(cfg, code, testOAuthVerifier, testOAuthReturnTo, now); err != nil || ok {
		t.Fatalf("consumed code replay after reopen = %v, %v", ok, err)
	}
}

func TestOAuthSessionCodeAccountUnavailable(t *testing.T) {
	for _, action := range []string{"suspend", "delete"} {
		t.Run(action, func(t *testing.T) {
			store := openTestStore(t)
			now := time.Now().UTC()
			code := issueTestOAuthCode(t, store, now)
			var err error
			if action == "suspend" {
				_, err = store.db.Exec(`UPDATE users SET suspended_at = ? WHERE id = ?`, formatTime(now), "github:code-user")
			} else {
				_, err = store.db.Exec(`INSERT INTO deleted_accounts(user_id, deleted_at) VALUES(?, ?)`, "github:code-user", formatTime(now))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, _, ok, err := store.redeemOAuthSessionCode(Config{SessionKey: "secret"}, code, testOAuthVerifier, testOAuthReturnTo, now); err != nil || ok {
				t.Fatalf("unavailable account redeem = %v, %v", ok, err)
			}
		})
	}
}

func TestOAuthSessionCodeMigrationAndPruning(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.db.Exec(`DROP TABLE oauth_session_codes; UPDATE schema_meta SET version = 20`); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(); err != nil {
		t.Fatal(err)
	}
	version, err := store.SchemaVersion()
	if err != nil || version != CurrentServerSchemaVersion {
		t.Fatalf("schema = %d, %v; want %d", version, err, CurrentServerSchemaVersion)
	}
	now := time.Now().UTC()
	issueTestOAuthCode(t, store, now.Add(-time.Minute))
	issueTestOAuthCode(t, store, now)
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM oauth_session_codes`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expired codes not pruned: %d, %v", count, err)
	}
}

func TestOAuthCrossOriginCodeFlow(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			writeJSON(w, map[string]string{"access_token": "provider-access"})
		} else {
			writeJSON(w, map[string]any{"id": 42, "email": "user@example.com"})
		}
	}))
	defer provider.Close()
	store := openTestStore(t)
	cfg := Config{SessionKey: "oauth-code-secret", AppOrigin: "https://app.example", OAuthProviders: map[string]OAuthProviderConfig{
		"github": {ClientID: "client", ClientSecret: "secret", RedirectURL: "https://api.example/v1/auth/github/callback", TokenURL: provider.URL + "/token", UserURL: provider.URL + "/user"},
	}}
	mux := NewMux(cfg, store)
	startURL := "https://api.example/v1/auth/github?returnTo=" + url.QueryEscape(testOAuthReturnTo)
	for _, query := range []string{"", "&code_challenge=bad&code_challenge_method=S256", "&code_challenge=" + pkceChallenge(testOAuthVerifier) + "&code_challenge_method=plain"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, startURL+query, nil))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("start without valid S256 status = %d", rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, startURL+"&code_challenge="+pkceChallenge(testOAuthVerifier)+"&code_challenge_method=S256", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("start status = %d: %s", rr.Code, rr.Body.String())
	}
	stateCookie := rr.Result().Cookies()[0]
	stateData, ok := parseOAuthStateCookie(stateCookie.Value)
	if !ok || stateData.ClientChallenge != pkceChallenge(testOAuthVerifier) {
		t.Fatal("client challenge was not carried through OAuth state")
	}
	location, _ := url.Parse(rr.Header().Get("Location"))
	if location.Query().Get("code_challenge") == stateData.ClientChallenge {
		t.Fatal("provider PKCE and client PKCE were mixed")
	}
	callback := httptest.NewRequest(http.MethodGet, "https://api.example/v1/auth/github/callback?code=provider-code&state="+stateData.State, nil)
	callback.AddCookie(stateCookie)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, callback)
	if rr.Code != http.StatusOK {
		t.Fatalf("callback status = %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "accessToken") || strings.Contains(rr.Body.String(), "refreshToken") || strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatal("cross-origin callback exposed session credentials")
	}
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == sessionRefreshCookie || cookie.Name == sessionCSRFCookie {
			t.Fatal("cross-origin callback installed session cookies")
		}
	}
	body := rr.Body.String()
	marker := `"code":"`
	_, tail, ok := strings.Cut(body, marker)
	if !ok {
		t.Fatal("callback did not deliver authorization code")
	}
	code, _, _ := strings.Cut(tail, `"`)
	if len(code) != 43 || !strings.Contains(body, "oauthCode=") || rr.Header().Get("Cache-Control") != "no-store" || rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("code handoff metadata missing")
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM refresh_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatal("session was created before PKCE redemption")
	}
	for _, tc := range []struct {
		name, verifier, origin string
		want                   int
	}{
		{"wrong origin", testOAuthVerifier, "https://evil.example", 403},
		{"wrong verifier", strings.Repeat("z", 43), "https://app.example", 401},
		{"correct", testOAuthVerifier, "https://app.example", 200},
		{"replay", testOAuthVerifier, "https://app.example", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"code": code, "codeVerifier": tc.verifier, "returnTo": testOAuthReturnTo})
			req := httptest.NewRequest(http.MethodPost, "https://api.example/v1/auth/session/code", strings.NewReader(string(input)))
			req.Header.Set("Origin", tc.origin)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			if tc.want == 200 {
				var pair struct{ AccessToken, RefreshToken, UserID string }
				if err := json.Unmarshal(rr.Body.Bytes(), &pair); err != nil || pair.UserID != "github:42" || pair.AccessToken == "" || pair.RefreshToken == "" {
					t.Fatalf("code session missing: %v", err)
				}
				if _, ok := verifySessionClaims(cfg, pair.RefreshToken, "refresh", time.Now()); !ok {
					t.Fatal("redeemed refresh token cannot rotate")
				}
				if len(rr.Result().Cookies()) != 0 {
					t.Fatal("encrypted-record exchange unexpectedly installed cookies")
				}
			}
		})
	}
}
