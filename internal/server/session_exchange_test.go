// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSessionCookiePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint         string
		dev, forwarded, secure bool
	}{
		{"hosted TLS", "https://cashflux.example/", false, false, true},
		{"hosted proxy TLS", "http://cashflux.example/", false, true, true},
		{"hosted plain HTTP fails secure", "http://cashflux.example/", false, false, true},
		{"localhost is secure by default", "http://localhost:8081/", false, false, true},
		{"explicit localhost dev", "http://localhost:8081/", true, false, false},
		{"explicit loopback dev", "http://127.0.0.1:8081/", true, false, false},
		{"explicit IPv6 dev", "http://[::1]:8081/", true, false, false},
		{"nonlocal dev stays secure", "http://cashflux.example/", true, false, true},
		{"localhost TLS dev", "https://localhost:8081/", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.endpoint, nil)
			if tc.forwarded {
				req.Header.Set("X-Forwarded-Proto", "https")
			}
			secure := sessionCookieSecure(Config{DevMode: tc.dev}, req)
			if secure != tc.secure {
				t.Fatalf("secure = %v, want %v", secure, tc.secure)
			}
			rr := httptest.NewRecorder()
			setRefreshCookie(rr, "refresh", secure, time.Now().Add(time.Hour))
			setCSRFCookieValue(rr, "csrf", secure, time.Now().Add(time.Hour))
			refresh := sessionResponseCookie(t, rr, sessionRefreshCookie, "/v1/auth/refresh")
			csrf := sessionResponseCookie(t, rr, sessionCSRFCookie, "/")
			if !refresh.HttpOnly || refresh.Secure != secure || refresh.SameSite != http.SameSiteStrictMode {
				t.Fatalf("refresh cookie flags = %+v", refresh)
			}
			if csrf.HttpOnly || csrf.Secure != secure || csrf.SameSite != http.SameSiteStrictMode {
				t.Fatalf("CSRF cookie flags = %+v", csrf)
			}
			for _, name := range []string{sessionRefreshCookie, sessionCSRFCookie} {
				old := sessionResponseCookie(t, rr, name, "/v1/auth")
				if old.MaxAge != -1 || old.Value != "" || !old.Expires.Before(time.Now()) {
					t.Fatalf("old cookie was not expired: %+v", old)
				}
			}
		})
	}
}

func sessionResponseCookie(t *testing.T, rr *httptest.ResponseRecorder, name, path string) *http.Cookie {
	t.Helper()
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == name && cookie.Path == path {
			return cookie
		}
	}
	t.Fatalf("missing %s at %s", name, path)
	return nil
}

func sessionExchangePair(t *testing.T, store *Store, cfg Config, user string) (string, string) {
	t.Helper()
	if err := store.UpsertUser(User{ID: user, Provider: "password", Subject: user, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	access, refresh, err := issueStoredSessionPair(cfg, store, user, time.Now(), "", "test device")
	if err != nil {
		t.Fatal(err)
	}
	return access, refresh
}

func sessionExchangeRequest(endpoint, access, csrf, refresh string) *http.Request {
	body, _ := json.Marshal(map[string]string{"refreshToken": refresh})
	r := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	r.Header.Set("Origin", "https://cashflux.example")
	r.Header.Set("Authorization", "Bearer "+access)
	if csrf != "" {
		r.Header.Set(sessionCSRFHeader, csrf)
		r.AddCookie(&http.Cookie{Name: sessionCSRFCookie, Value: csrf})
	}
	return r
}

func TestSessionExchangeReloadAndLogout(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{SessionKey: "session-exchange-secret", AppOrigin: "https://cashflux.example"}
	access, refresh := sessionExchangePair(t, store, cfg, "reload-account")
	mux := NewMux(cfg, store)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := url.Parse("https://cashflux.example/settings")
	auth, _ := url.Parse("https://cashflux.example/v1/auth/session/csrf")
	// Seed the legacy path cookies to prove migration removes duplicates.
	jar.SetCookies(auth, []*http.Cookie{{Name: sessionCSRFCookie, Value: "legacy", Path: "/v1/auth"}, {Name: sessionRefreshCookie, Value: "legacy", Path: "/v1/auth"}})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, sessionExchangeRequest(auth.String(), access, "", ""))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("bootstrap status = %d: %s", rr.Code, rr.Body.String())
	}
	csrf := rr.Header().Get(sessionCSRFHeader)
	if csrf == "" || sessionResponseCookie(t, rr, sessionCSRFCookie, "/").HttpOnly {
		t.Fatal("bootstrap did not publish a readable CSRF cookie")
	}
	jar.SetCookies(auth, rr.Result().Cookies())
	endpoint := "https://cashflux.example/v1/auth/session/cookie"
	rr = httptest.NewRecorder()
	exchange := sessionExchangeRequest(endpoint, access, csrf, refresh)
	for _, cookie := range jar.Cookies(auth) {
		if cookie.Name == sessionDeviceBindingCookie {
			exchange.AddCookie(cookie)
		}
	}
	mux.ServeHTTP(rr, exchange)
	if rr.Code != http.StatusOK {
		t.Fatalf("exchange status = %d: %s", rr.Code, rr.Body.String())
	}
	var response oauthSessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	rotated := sessionResponseCookie(t, rr, sessionRefreshCookie, "/v1/auth/refresh")
	if rotated.Value == refresh || response.UserID != "reload-account" || response.AccessToken == "" || strings.Contains(rr.Body.String(), "refreshToken") || strings.Contains(rr.Body.String(), rotated.Value) {
		t.Fatal("exchange did not rotate privately for the authenticated account")
	}
	old, _ := verifySessionClaims(cfg, refresh, "refresh", time.Now())
	stored, ok, err := store.GetRefreshSession(old.JTI)
	if err != nil || !ok || stored.UsedAt.IsZero() || stored.SuccessorJTI == "" {
		t.Fatalf("input was not durably consumed: %v, %v", ok, err)
	}
	jar.SetCookies(auth, rr.Result().Cookies())
	// A duplicate conversion with the same browser binding receives the same
	// successor inside the established reuse grace, without exposing it in JSON.
	retry := sessionExchangeRequest(endpoint, access, csrf, refresh)
	for _, cookie := range jar.Cookies(auth) {
		if cookie.Name == sessionDeviceBindingCookie {
			retry.AddCookie(cookie)
		}
	}
	retryRR := httptest.NewRecorder()
	mux.ServeHTTP(retryRR, retry)
	if retryRR.Code != http.StatusOK || sessionResponseCookie(t, retryRR, sessionRefreshCookie, "/v1/auth/refresh").Value != rotated.Value || strings.Contains(retryRR.Body.String(), rotated.Value) {
		t.Fatalf("conversion retry status = %d: %s", retryRR.Code, retryRR.Body.String())
	}
	// Drop all page token memory. The SPA can read CSRF at /settings and then
	// bootstrap using only the scoped refresh cookie plus double-submit CSRF.
	rootCookies := jar.Cookies(root)
	if len(rootCookies) != 1 || rootCookies[0].Name != sessionCSRFCookie || rootCookies[0].Value != csrf {
		t.Fatalf("root reload cookies = %v", rootCookies)
	}
	refreshURL, _ := url.Parse("https://cashflux.example/v1/auth/refresh")
	req := httptest.NewRequest(http.MethodPost, refreshURL.String(), nil)
	req.Header.Set("Origin", "https://cashflux.example")
	req.Header.Set(sessionCSRFHeader, rootCookies[0].Value)
	for _, cookie := range jar.Cookies(refreshURL) {
		req.AddCookie(cookie)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("reload refresh status = %d: %s", rr.Code, rr.Body.String())
	}
	jar.SetCookies(refreshURL, rr.Result().Cookies())
	list := sessionExchangeRequest("https://cashflux.example/v1/auth/refresh/sessions", access, csrf, "")
	list.Method = http.MethodGet
	for _, cookie := range jar.Cookies(refreshURL) {
		if cookie.Name != sessionCSRFCookie {
			list.AddCookie(cookie)
		}
	}
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, list)
	var sessions oauthSessionsResponse
	if err := json.Unmarshal(listRR.Body.Bytes(), &sessions); err != nil || listRR.Code != http.StatusOK || len(sessions.Sessions) != 1 || !sessions.Sessions[0].Current {
		t.Fatalf("cookie-scoped session listing status = %d: %s (%v)", listRR.Code, listRR.Body.String(), err)
	}
	logoutURL, _ := url.Parse("https://cashflux.example/v1/auth/refresh/logout")
	req = httptest.NewRequest(http.MethodPost, logoutURL.String(), nil)
	req.Header.Set("Origin", "https://cashflux.example")
	req.Header.Set(sessionCSRFHeader, csrf)
	for _, cookie := range jar.Cookies(logoutURL) {
		req.AddCookie(cookie)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", rr.Code, rr.Body.String())
	}
	for _, tc := range []struct{ name, path string }{
		{sessionRefreshCookie, "/v1/auth/refresh"}, {sessionRefreshCookie, "/v1/auth"},
		{sessionCSRFCookie, "/"}, {sessionCSRFCookie, "/v1/auth"},
	} {
		cookie := sessionResponseCookie(t, rr, tc.name, tc.path)
		if cookie.MaxAge != -1 || !cookie.Secure || cookie.Value != "" || !cookie.Expires.Before(time.Now()) {
			t.Fatalf("logout cookie not expired securely: %+v", cookie)
		}
	}
	jar.SetCookies(logoutURL, rr.Result().Cookies())
	for _, cookie := range jar.Cookies(refreshURL) {
		if cookie.Name == sessionRefreshCookie || cookie.Name == sessionCSRFCookie {
			t.Fatalf("cookie survived logout: %v", cookie)
		}
	}
	families, err := store.ListRefreshSessionFamilies("reload-account", time.Now())
	if err != nil || len(families) != 0 {
		t.Fatalf("logout left an active family: %v, %v", families, err)
	}
}

func TestSessionExchangeGuards(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{SessionKey: "session-exchange-secret", Token: "static", AppOrigin: "https://other.example"}
	access, refresh := sessionExchangePair(t, store, cfg, "account-a")
	_, foreign := sessionExchangePair(t, store, cfg, "account-b")
	for _, tc := range []struct {
		name   string
		change func(*http.Request)
		want   int
	}{
		{"no access", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"refresh as access", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+refresh) }, 401},
		{"static bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer static") }, 401},
		{"no CSRF header", func(r *http.Request) { r.Header.Del(sessionCSRFHeader) }, 403},
		{"no CSRF cookie", func(r *http.Request) { r.Header.Del("Cookie") }, 403},
		{"wrong CSRF", func(r *http.Request) { r.Header.Set(sessionCSRFHeader, "wrong") }, 403},
		{"no origin", func(r *http.Request) { r.Header.Del("Origin") }, 403},
		{"allowed cross-origin", func(r *http.Request) { r.Header.Set("Origin", "https://other.example") }, 403},
		{"wrong scheme", func(r *http.Request) { r.Header.Set("Origin", "http://cashflux.example") }, 403},
		{"origin with path", func(r *http.Request) { r.Header.Set("Origin", "https://cashflux.example/evil") }, 403},
		{"insecure transport", func(r *http.Request) { r.TLS = nil; r.Header.Set("Origin", "http://cashflux.example") }, 403},
		{"other account refresh", func(r *http.Request) {
			b, _ := json.Marshal(map[string]string{"refreshToken": foreign})
			r.Body = io.NopCloser(strings.NewReader(string(b)))
		}, 401},
		{"invalid JSON", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader("{")) }, 400},
		{"trailing JSON", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{} {}`)) }, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := sessionExchangeRequest("https://cashflux.example/v1/auth/session/cookie", access, "csrf", refresh)
			tc.change(r)
			rr := httptest.NewRecorder()
			handleSessionCookie(cfg, store)(rr, r)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			if len(rr.Result().Cookies()) != 0 {
				t.Fatal("failed exchange set cookies")
			}
		})
	}
}

func TestSessionRoutesRateLimited(t *testing.T) {
	for _, route := range []string{"/v1/auth/session/csrf", "/v1/auth/session/cookie", "/v1/auth/session/code", "/v1/auth/refresh/logout"} {
		t.Run(route, func(t *testing.T) {
			h := NewMux(Config{AuthRateLimitPerMinute: 1}, openTestStore(t))
			for attempt := 0; attempt < 2; attempt++ {
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "https://cashflux.example"+route, strings.NewReader(`{}`)))
				if attempt == 1 && rr.Code != http.StatusTooManyRequests {
					t.Fatalf("second request status = %d, want 429", rr.Code)
				}
			}
		})
	}
}

func TestSessionCSRFBootstrapGuardsAndPreservesToken(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{SessionKey: "session-exchange-secret"}
	access, _ := sessionExchangePair(t, store, cfg, "csrf-account")
	for _, tc := range []struct {
		name   string
		change func(*http.Request)
		want   int
	}{
		{"valid bootstrap", func(*http.Request) {}, 204},
		{"no bearer", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"no origin", func(r *http.Request) { r.Header.Del("Origin") }, 403},
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "https://elsewhere.example") }, 403},
		{"insecure", func(r *http.Request) { r.TLS = nil; r.Header.Set("Origin", "http://cashflux.example") }, 403},
		{"forged forwarded HTTPS", func(r *http.Request) { r.TLS = nil; r.Header.Set("X-Forwarded-Proto", "https") }, 403},
		{"default port origin", func(r *http.Request) { r.Header.Set("Origin", "https://cashflux.example:443") }, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := sessionExchangeRequest("https://cashflux.example/v1/auth/session/csrf", access, "existing-csrf", "")
			tc.change(req)
			rr := httptest.NewRecorder()
			handleSessionCSRF(cfg, store)(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("bootstrap status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			if tc.want == 204 {
				if rr.Header().Get(sessionCSRFHeader) != "existing-csrf" || sessionResponseCookie(t, rr, sessionCSRFCookie, "/").Value != "existing-csrf" {
					t.Fatal("bootstrap replaced another tab's CSRF")
				}
				device := sessionResponseCookie(t, rr, sessionDeviceBindingCookie, "/v1/auth")
				if device.Value == "" || !device.HttpOnly || !device.Secure {
					t.Fatal("bootstrap did not prepare the bound rotation retry")
				}
			} else if len(rr.Result().Cookies()) != 0 {
				t.Fatal("rejected bootstrap installed cookies")
			}
		})
	}
	_, proxy, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	cfg.TrustedProxies = []*net.IPNet{proxy}
	req := sessionExchangeRequest("http://cashflux.example/v1/auth/session/csrf", access, "", "")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	handleSessionCSRF(cfg, store)(rr, req)
	if rr.Code != http.StatusNoContent || !sessionResponseCookie(t, rr, sessionCSRFCookie, "/").Secure {
		t.Fatalf("trusted TLS proxy bootstrap status = %d", rr.Code)
	}
}

func TestSessionExchangeRefusesUnavailableAccounts(t *testing.T) {
	for _, action := range []string{"suspended", "deleted", "revoked", "expired access"} {
		t.Run(action, func(t *testing.T) {
			store := openTestStore(t)
			cfg := Config{SessionKey: "session-exchange-secret"}
			access, refresh := sessionExchangePair(t, store, cfg, "unavailable-account")
			var err error
			want := http.StatusUnauthorized
			switch action {
			case "suspended":
				_, err = store.db.Exec(`UPDATE users SET suspended_at = ? WHERE id = ?`, formatTime(time.Now()), "unavailable-account")
				want = http.StatusForbidden
			case "deleted":
				_, err = store.db.Exec(`INSERT INTO deleted_accounts(user_id, deleted_at) VALUES(?, ?)`, "unavailable-account", formatTime(time.Now()))
			case "revoked":
				claims, _ := verifySessionClaims(cfg, refresh, "refresh", time.Now())
				err = store.RevokeRefreshSessionFamily(claims.Family, time.Now())
			case "expired access":
				access, err = issueSessionToken(cfg, "unavailable-account", "access", sessionAccessTTL, time.Now().Add(-time.Hour))
			}
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handleSessionCookie(cfg, store)(rr, sessionExchangeRequest("https://cashflux.example/v1/auth/session/cookie", access, "csrf", refresh))
			if rr.Code != want {
				t.Fatalf("unavailable account status = %d, want %d: %s", rr.Code, want, rr.Body.String())
			}
			for _, cookie := range rr.Result().Cookies() {
				if cookie.Name == sessionRefreshCookie && cookie.Value != "" {
					t.Fatal("unavailable account received refresh credential")
				}
			}
		})
	}
}

func TestSessionLogoutDoesNotAcknowledgeFailedRevocation(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{SessionKey: "session-exchange-secret"}
	_, refresh := sessionExchangePair(t, store, cfg, "logout-account")
	if _, err := store.db.Exec(`CREATE TRIGGER reject_logout BEFORE UPDATE ON refresh_tokens BEGIN SELECT RAISE(ABORT, 'test revoke failure'); END`); err != nil {
		t.Fatal(err)
	}
	req := sessionExchangeRequest("https://cashflux.example/v1/auth/refresh/logout", "", "csrf", "")
	req.AddCookie(&http.Cookie{Name: sessionRefreshCookie, Value: refresh})
	rr := httptest.NewRecorder()
	handleOAuthLogout(cfg, store)(rr, req)
	if rr.Code != http.StatusInternalServerError || len(rr.Result().Cookies()) != 0 {
		t.Fatalf("failed revocation status/cookies = %d/%v", rr.Code, rr.Result().Cookies())
	}
}
