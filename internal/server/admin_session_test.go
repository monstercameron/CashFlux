// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func createAdminSessionUser(t *testing.T, store *Store, username, password, role string) User {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	recoveryHash, err := bcrypt.GenerateFromPassword([]byte("recovery-"+username), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateLocalUserWithRole(username, string(passwordHash), string(recoveryHash), role, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func adminSessionRequest(t *testing.T, h http.Handler, method, requestPath, body string, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "https://budget.example"+requestPath, bytes.NewBufferString(body))
	req.Host = "budget.example"
	req.Header.Set("Origin", "https://budget.example")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set(sessionCSRFHeader, csrf)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func cookiesByName(cookies []*http.Cookie) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		out[cookie.Name] = cookie
	}
	return out
}

func loginAdminSession(t *testing.T, h http.Handler, username, password string) (map[string]*http.Cookie, adminSessionResponse) {
	t.Helper()
	body, _ := json.Marshal(adminSessionLoginRequest{Username: username, Password: password})
	rr := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/login", string(body), nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login status = %d body %q", rr.Code, rr.Body.String())
	}
	var session adminSessionResponse
	if err := json.NewDecoder(rr.Body).Decode(&session); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return cookiesByName(rr.Result().Cookies()), session
}

func TestAdminOwnerCredentialSessionAndCookieSecurity(t *testing.T) {
	store := openTestStore(t)
	owner := createAdminSessionUser(t, store, "cam", "owner-password", RoleOwner)
	cfg := Config{AuthMode: "token", Token: "break-glass", SessionKey: "admin-session-test-key"}
	h := NewMux(cfg, store)

	cookies, session := loginAdminSession(t, h, "cam", "owner-password")
	if !session.Authenticated || session.UserID != owner.ID || session.Username != "cam" || session.Role != RoleOwner {
		t.Fatalf("session = %+v", session)
	}
	for _, tc := range []struct {
		name     string
		path     string
		httpOnly bool
	}{
		{name: adminAccessCookie, path: adminCookiePath, httpOnly: true},
		{name: adminRefreshCookie, path: adminRefreshPath, httpOnly: true},
		{name: adminCSRFCookie, path: "/", httpOnly: false},
	} {
		cookie := cookies[tc.name]
		if cookie == nil {
			t.Fatalf("missing %s cookie", tc.name)
		}
		if cookie.Path != tc.path || cookie.HttpOnly != tc.httpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("%s cookie = %+v", tc.name, cookie)
		}
	}
	if got := cookies[adminAccessCookie].Value; got == "" {
		t.Fatal("access cookie is empty")
	}
	if got := cookies[adminRefreshCookie].Value; got == "" {
		t.Fatal("refresh cookie is empty")
	}

	all := []*http.Cookie{cookies[adminAccessCookie], cookies[adminRefreshCookie], cookies[adminCSRFCookie]}
	overview := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/overview", "", all, "")
	if overview.Code != http.StatusOK {
		t.Fatalf("cookie overview = %d body %q", overview.Code, overview.Body.String())
	}
	audit := adminSessionRequest(t, h, http.MethodGet, "/v1/audit", "", all, "")
	if audit.Code != http.StatusOK {
		t.Fatalf("cookie audit feed = %d body %q", audit.Code, audit.Body.String())
	}

	status := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/session", "", all, "")
	if status.Code != http.StatusOK || status.Header().Get(sessionCSRFHeader) != cookies[adminCSRFCookie].Value {
		t.Fatalf("session status = %d csrf %q body %q", status.Code, status.Header().Get(sessionCSRFHeader), status.Body.String())
	}

	bearerReq := httptest.NewRequest(http.MethodGet, "/v1/admin/overview", nil)
	bearerReq.Header.Set("Authorization", "Bearer "+cookies[adminAccessCookie].Value)
	bearer := httptest.NewRecorder()
	h.ServeHTTP(bearer, bearerReq)
	if bearer.Code != http.StatusOK {
		t.Fatalf("owner bearer overview = %d body %q", bearer.Code, bearer.Body.String())
	}
}

func TestAdminCookieMutationRequiresCSRFAndSameOrigin(t *testing.T) {
	store := openTestStore(t)
	createAdminSessionUser(t, store, "cam", "owner-password", RoleOwner)
	cfg := Config{AuthMode: "token", Token: "break-glass", SessionKey: "admin-session-test-key"}
	h := NewMux(cfg, store)
	cookies, _ := loginAdminSession(t, h, "cam", "owner-password")
	all := []*http.Cookie{cookies[adminAccessCookie], cookies[adminRefreshCookie], cookies[adminCSRFCookie]}
	body := `{"username":"new-user","password":"new-user-password","role":"member"}`

	withoutCSRF := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/users", body, all, "")
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("without csrf = %d body %q", withoutCSRF.Code, withoutCSRF.Body.String())
	}

	withCSRF := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/users", body, all, cookies[adminCSRFCookie].Value)
	if withCSRF.Code != http.StatusOK {
		t.Fatalf("with csrf = %d body %q", withCSRF.Code, withCSRF.Body.String())
	}

	hostileReq := httptest.NewRequest(http.MethodPost, "https://budget.example/v1/admin/users", bytes.NewBufferString(body))
	hostileReq.Host = "budget.example"
	hostileReq.Header.Set("Origin", "https://evil.example")
	hostileReq.Header.Set("Content-Type", "application/json")
	hostileReq.Header.Set(sessionCSRFHeader, cookies[adminCSRFCookie].Value)
	for _, cookie := range all {
		hostileReq.AddCookie(cookie)
	}
	hostile := httptest.NewRecorder()
	h.ServeHTTP(hostile, hostileReq)
	if hostile.Code != http.StatusForbidden {
		t.Fatalf("hostile origin = %d body %q", hostile.Code, hostile.Body.String())
	}
}

func TestAdminSessionRejectsNonOwnerAndWrongPassword(t *testing.T) {
	store := openTestStore(t)
	member := createAdminSessionUser(t, store, "member", "member-password", RoleMember)
	cfg := Config{AuthMode: "token", Token: "break-glass", SessionKey: "admin-session-test-key"}
	h := NewMux(cfg, store)

	body, _ := json.Marshal(adminSessionLoginRequest{Username: "member", Password: "member-password"})
	rr := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/login", string(body), nil, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("member login = %d body %q", rr.Code, rr.Body.String())
	}
	for _, cookie := range rr.Result().Cookies() {
		if (cookie.Name == adminAccessCookie || cookie.Name == adminRefreshCookie) && cookie.Value != "" {
			t.Fatalf("member received usable %s cookie", cookie.Name)
		}
	}
	families, err := store.ListRefreshSessionFamilies(member.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 0 {
		t.Fatalf("member retained refresh families: %+v", families)
	}

	badBody, _ := json.Marshal(adminSessionLoginRequest{Username: "member", Password: "wrong-password"})
	bad := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/login", string(badBody), nil, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d body %q", bad.Code, bad.Body.String())
	}
}

func TestAdminExplicitIDCompatibilityAndOwnerDemotion(t *testing.T) {
	store := openTestStore(t)
	owner := createAdminSessionUser(t, store, "cam", "owner-password", RoleOwner)
	member := createAdminSessionUser(t, store, "legacy-admin", "member-password", RoleMember)
	cfg := Config{
		AuthMode:     "token",
		Token:        "break-glass",
		SessionKey:   "admin-session-test-key",
		AdminUserIDs: []string{member.ID},
	}
	h := NewMux(cfg, store)

	if _, session := loginAdminSession(t, h, "legacy-admin", "member-password"); session.UserID != member.ID {
		t.Fatalf("explicit admin session = %+v", session)
	}

	cookies, _ := loginAdminSession(t, h, "cam", "owner-password")
	if err := store.SetUserRole(owner.ID, RoleMember); err != nil {
		t.Fatal(err)
	}
	all := []*http.Cookie{cookies[adminAccessCookie], cookies[adminRefreshCookie], cookies[adminCSRFCookie]}
	overview := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/overview", "", all, "")
	if overview.Code != http.StatusForbidden {
		t.Fatalf("demoted owner overview = %d body %q", overview.Code, overview.Body.String())
	}
}

func TestAdminSessionRefreshAndLogoutRevokeFamily(t *testing.T) {
	store := openTestStore(t)
	createAdminSessionUser(t, store, "cam", "owner-password", RoleOwner)
	cfg := Config{AuthMode: "token", Token: "break-glass", SessionKey: "admin-session-test-key"}
	h := NewMux(cfg, store)
	cookies, _ := loginAdminSession(t, h, "cam", "owner-password")
	initialRefresh := cookies[adminRefreshCookie]
	initialCSRF := cookies[adminCSRFCookie]

	refresh := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/refresh", "", []*http.Cookie{initialRefresh, initialCSRF}, initialCSRF.Value)
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh = %d body %q", refresh.Code, refresh.Body.String())
	}
	rotated := cookiesByName(refresh.Result().Cookies())
	if rotated[adminRefreshCookie] == nil || rotated[adminRefreshCookie].Value == initialRefresh.Value {
		t.Fatal("refresh cookie was not rotated")
	}

	logoutCookies := []*http.Cookie{rotated[adminAccessCookie], rotated[adminRefreshCookie], rotated[adminCSRFCookie]}
	logout := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/logout", "", logoutCookies, rotated[adminCSRFCookie].Value)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout = %d body %q", logout.Code, logout.Body.String())
	}

	afterLogout := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/session/refresh", "", []*http.Cookie{rotated[adminRefreshCookie], rotated[adminCSRFCookie]}, rotated[adminCSRFCookie].Value)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d body %q", afterLogout.Code, afterLogout.Body.String())
	}
}
