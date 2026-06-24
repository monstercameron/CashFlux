// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// adminReq builds an admin-authenticated request against the test mux.
func adminReq(t *testing.T, mux http.Handler, method, path, adminToken, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", adminBearer(adminToken))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestAdminUserDetail(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store)

	w := adminReq(t, mux, http.MethodGet, "/v1/admin/users/ua", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminUserDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "ua" || resp.Email != "alice@example.com" {
		t.Errorf("identity = %+v", resp)
	}
	if resp.SubscriptionPlan != "monthly" || resp.SubscriptionStatus != "active" {
		t.Errorf("subscription = %q/%q, want monthly/active", resp.SubscriptionPlan, resp.SubscriptionStatus)
	}
	if resp.UsageTodayRequests != 5 || resp.UsageTodayTokens != 100 {
		t.Errorf("usage = %d/%d, want 5/100", resp.UsageTodayRequests, resp.UsageTodayTokens)
	}
	// No secrets must leak through the detail view.
	for _, forbidden := range []string{"ciphertext", "nonce", "token", "dataset"} {
		if containsBytes(w.Body.Bytes(), forbidden) {
			t.Errorf("detail JSON leaks %q", forbidden)
		}
	}
}

func TestAdminUserDetailNotFound(t *testing.T) {
	adminToken := "admin-secret"
	mux, _ := newAdminTestMux(t, resolvedAdminID(adminToken))
	w := adminReq(t, mux, http.MethodGet, "/v1/admin/users/ghost", adminToken, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestAdminUserDetailNonAdmin(t *testing.T) {
	token := "admin-secret"
	store := openTestStore(t)
	seedAdminFixture(t, store)
	cfg := Config{Addr: ":0", DataDir: t.TempDir(), AuthMode: "token", Token: token, AdminUserIDs: nil, Metrics: NewMetrics()}
	mux := NewMux(cfg, store)
	w := adminReq(t, mux, http.MethodGet, "/v1/admin/users/ua", token, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestAdminUserUsageHistory(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	if err := store.UpsertUser(User{ID: "uh", Provider: "github", Subject: "h", Email: "h@e.com", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		if _, err := store.AddUsage("uh", now.AddDate(0, 0, -i), int64(i+1), int64((i+1)*10)); err != nil {
			t.Fatal(err)
		}
	}
	w := adminReq(t, mux, http.MethodGet, "/v1/admin/users/uh/usage?days=7", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminUsageHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 7 {
		t.Errorf("days = %d, want 7", resp.Days)
	}
	if len(resp.Usage) != 4 {
		t.Errorf("usage rows = %d, want 4", len(resp.Usage))
	}
	// Newest-first ordering.
	if len(resp.Usage) >= 2 && resp.Usage[0].Day < resp.Usage[1].Day {
		t.Errorf("usage not newest-first: %+v", resp.Usage)
	}
}

func TestAdminUserSetPlan(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store) // ua: monthly/active

	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/ua/plan", adminToken, `{"plan":"annual","status":"trialing"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	sub, ok, err := store.GetSubscription("ua")
	if err != nil || !ok {
		t.Fatalf("GetSubscription: ok=%v err=%v", ok, err)
	}
	if sub.Plan != "annual" || sub.Status != "trialing" {
		t.Errorf("subscription = %q/%q, want annual/trialing", sub.Plan, sub.Status)
	}
}

func TestAdminUserSetPlanNoSubscription(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	if err := store.UpsertUser(User{ID: "uns", Provider: "github", Subject: "n", Email: "n@e.com", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/uns/plan", adminToken, `{"plan":"annual"}`)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", w.Code)
	}
}

func TestAdminUserRevokeSessions(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store)
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/ua/revoke-sessions", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK || resp.Action != "revokeSessions" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestAdminUserDelete(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store)

	w := adminReq(t, mux, http.MethodDelete, "/v1/admin/users/ub", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if _, found, err := store.GetUserByID("ub"); err != nil || found {
		t.Errorf("user ub still present after delete (found=%v err=%v)", found, err)
	}
	// Deleting again → 404.
	w2 := adminReq(t, mux, http.MethodDelete, "/v1/admin/users/ub", adminToken, "")
	if w2.Code != http.StatusNotFound {
		t.Errorf("re-delete status = %d, want 404", w2.Code)
	}
}

func TestAdminUserDeleteSelfBlocked(t *testing.T) {
	adminToken := "admin-secret"
	adminID := resolvedAdminID(adminToken)
	mux, _ := newAdminTestMux(t, adminID)
	w := adminReq(t, mux, http.MethodDelete, "/v1/admin/users/"+adminID, adminToken, "")
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("self-delete status = %d, want 412", w.Code)
	}
}

func TestAdminManageUnauthenticated(t *testing.T) {
	mux, _ := newAdminTestMux(t, resolvedAdminID("admin-secret"))
	for _, path := range []string{"/v1/admin/users/ua", "/v1/admin/users/ua/usage"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", path, w.Code)
		}
	}
}
