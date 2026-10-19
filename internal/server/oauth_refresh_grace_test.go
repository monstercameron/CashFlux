// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthRefreshReuseGraceReturnsSamePairThenRevokes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		lateReuse    bool
		deviceBound  bool
		hashMismatch bool
		wantGrace    bool
	}{
		{name: "bound within grace", deviceBound: true, wantGrace: true},
		{name: "unbound replay"},
		{name: "after grace", lateReuse: true, deviceBound: true},
		{name: "token hash mismatch", deviceBound: true, hashMismatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := openTestStore(t)
			now := time.Now().UTC()
			cfg := Config{AuthMode: "oauth", AppOrigin: "http://127.0.0.1:8080", MasterKey: "0123456789abcdef0123456789abcdef"}
			if err := store.UpsertUser(User{ID: "github:refresh-grace", Provider: "github", Subject: "refresh-grace", CreatedAt: now}); err != nil {
				t.Fatalf("UpsertUser: %v", err)
			}
			_, originalRefresh, err := issueStoredSessionPair(cfg, store, "github:refresh-grace", now, "", "test device")
			if err != nil {
				t.Fatalf("issueStoredSessionPair: %v", err)
			}
			claims, ok := verifySessionClaims(cfg, originalRefresh, "refresh", now)
			if !ok {
				t.Fatal("original refresh token did not verify")
			}

			csrfRecorder := httptest.NewRecorder()
			csrf, err := setCSRFCookie(csrfRecorder, false, now.Add(sessionRefreshTTL))
			if err != nil {
				t.Fatalf("setCSRFCookie: %v", err)
			}
			var csrfCookie *http.Cookie
			for _, cookie := range csrfRecorder.Result().Cookies() {
				if cookie.Name == sessionCSRFCookie {
					csrfCookie = cookie
				}
			}
			if csrfCookie == nil {
				t.Fatal("csrf cookie missing")
			}
			if csrfCookie.Value != csrf {
				t.Fatal("csrf cookie value does not match returned token")
			}

			handler := handleOAuthRefresh(cfg, store)
			first := oauthRefreshRequest(t, handler, originalRefresh, csrfCookie)
			if first.Code != http.StatusOK {
				t.Fatalf("first refresh status = %d body %q", first.Code, first.Body.String())
			}
			var firstResponse oauthSessionResponse
			if err := json.Unmarshal(first.Body.Bytes(), &firstResponse); err != nil {
				t.Fatalf("decode first refresh: %v", err)
			}
			firstRefresh := refreshCookieFromResponse(t, first)
			if firstResponse.AccessToken == "" || firstRefresh.Value == "" {
				t.Fatalf("first refresh pair is incomplete: response=%+v cookie=%+v", firstResponse, firstRefresh)
			}
			if !firstRefresh.HttpOnly || firstRefresh.Path != "/v1/auth/refresh" || firstRefresh.SameSite != http.SameSiteStrictMode {
				t.Fatalf("refresh cookie attributes changed: %+v", firstRefresh)
			}

			if tc.lateReuse {
				if _, err := store.db.Exec(`UPDATE refresh_tokens SET used_at = ? WHERE jti = ?`, formatTime(time.Now().UTC().Add(-31*time.Second)), claims.JTI); err != nil {
					t.Fatalf("age consumed token: %v", err)
				}
			}
			var deviceCookie *http.Cookie
			for _, cookie := range first.Result().Cookies() {
				if cookie.Name == sessionDeviceBindingCookie {
					deviceCookie = cookie
				}
			}
			if deviceCookie == nil || !deviceCookie.HttpOnly || deviceCookie.Path != "/v1/auth" {
				t.Fatalf("device binding cookie missing or insecure: %+v", deviceCookie)
			}
			if !tc.deviceBound {
				deviceCookie = nil
			}
			presentedRefresh := originalRefresh
			if tc.hashMismatch {
				claims.Exp++
				presentedRefresh, err = issueSessionTokenWithClaims(cfg, claims)
				if err != nil {
					t.Fatalf("issue hash-mismatched token: %v", err)
				}
			}
			second := oauthRefreshRequest(t, handler, presentedRefresh, csrfCookie, deviceCookie)
			if !tc.wantGrace {
				if second.Code != http.StatusUnauthorized {
					t.Fatalf("rejected reuse status = %d body %q, want 401", second.Code, second.Body.String())
				}
				assertHTTPErrorReason(t, second, ErrorReasonUnauthenticated)
				reuseSuccessor := oauthRefreshRequest(t, handler, firstRefresh.Value, csrfCookie, deviceCookie)
				if reuseSuccessor.Code != http.StatusUnauthorized {
					t.Fatalf("successor after late reuse status = %d body %q, want 401", reuseSuccessor.Code, reuseSuccessor.Body.String())
				}
				return
			}

			if second.Code != http.StatusOK {
				t.Fatalf("duplicate inside grace status = %d body %q", second.Code, second.Body.String())
			}
			var secondResponse oauthSessionResponse
			if err := json.Unmarshal(second.Body.Bytes(), &secondResponse); err != nil {
				t.Fatalf("decode duplicate refresh: %v", err)
			}
			secondRefresh := refreshCookieFromResponse(t, second)
			if secondResponse.AccessToken == firstResponse.AccessToken || secondRefresh.Value != firstRefresh.Value {
				t.Fatalf("grace replay must mint fresh access and keep successor refresh: access_equal=%v refresh_equal=%v", secondResponse.AccessToken == firstResponse.AccessToken, secondRefresh.Value == firstRefresh.Value)
			}
		})
	}
}

func oauthRefreshRequest(t *testing.T, handler http.Handler, refresh string, csrf *http.Cookie, device ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set(sessionCSRFHeader, csrf.Value)
	req.AddCookie(csrf)
	if len(device) != 0 && device[0] != nil {
		req.AddCookie(device[0])
	}
	req.AddCookie(&http.Cookie{Name: sessionRefreshCookie, Value: refresh})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func refreshCookieFromResponse(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == sessionRefreshCookie && cookie.Path == "/v1/auth/refresh" {
			return cookie
		}
	}
	t.Fatal("refresh cookie missing from response")
	return nil
}
