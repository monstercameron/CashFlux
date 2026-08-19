// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
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

func TestAdminCreateUserCanLogIn(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users", adminToken,
		`{"username":"priya","password":"correct-horse-battery","role":"viewer"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var created AdminCreateUserResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID != "local:priya" || created.Username != "priya" || created.Role != RoleViewer {
		t.Fatalf("created = %+v", created)
	}
	if strings.TrimSpace(created.RecoveryCode) == "" {
		t.Fatal("recovery code is empty")
	}
	user, passwordHash, ok, err := store.GetLocalUserByUsername("priya")
	if err != nil || !ok {
		t.Fatalf("lookup created user: ok=%v err=%v", ok, err)
	}
	if user.ID != created.ID {
		t.Fatalf("user id = %q, want %q", user.ID, created.ID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("correct-horse-battery")); err != nil {
		t.Fatalf("created password did not verify: %v", err)
	}
	if role, err := store.UserRole(created.ID); err != nil || role != RoleViewer {
		t.Fatalf("role = %q err=%v, want viewer", role, err)
	}
	if w := adminReq(t, mux, http.MethodPost, "/v1/admin/users", adminToken,
		`{"username":"priya","password":"another-correct-password","role":"member"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUpdateUserIdentityIsAtomic(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	first, err := store.CreateLocalUserWithRole("marcus", "bcrypt-hash", "recovery-hash", RoleMember, time.Now().UTC())
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := store.CreateLocalUserWithRole("taken", "bcrypt-hash", "recovery-hash", RoleMember, time.Now().UTC()); err != nil {
		t.Fatalf("create second: %v", err)
	}
	w := adminReq(t, mux, http.MethodPatch, "/v1/admin/users/"+first.ID, adminToken,
		`{"username":"marcus-updated","role":"viewer"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	updated, ok, err := store.GetUserByID(first.ID)
	if err != nil || !ok {
		t.Fatalf("get updated user: ok=%v err=%v", ok, err)
	}
	if updated.Username != "marcus-updated" || updated.Role != RoleViewer {
		t.Fatalf("updated = %+v", updated)
	}
	w = adminReq(t, mux, http.MethodPatch, "/v1/admin/users/"+first.ID, adminToken,
		`{"username":"taken","role":"owner"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("collision status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	afterCollision, _, err := store.GetUserByID(first.ID)
	if err != nil {
		t.Fatalf("get after collision: %v", err)
	}
	if afterCollision.Username != "marcus-updated" || afterCollision.Role != RoleViewer {
		t.Fatalf("collision partially updated identity: %+v", afterCollision)
	}
}

func TestAdminUpdateRejectsSelfDemotion(t *testing.T) {
	adminToken := "admin-secret"
	adminID := resolvedAdminID(adminToken)
	mux, store := newAdminTestMux(t, adminID)
	if err := store.UpsertUser(User{ID: adminID, Provider: "token", Subject: adminID, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create admin row: %v", err)
	}
	if err := store.SetUserRole(adminID, RoleOwner); err != nil {
		t.Fatalf("set owner role: %v", err)
	}
	w := adminReq(t, mux, http.MethodPatch, "/v1/admin/users/"+adminID, adminToken, `{"role":"member"}`)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412; body: %s", w.Code, w.Body.String())
	}
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
	store := openTestStore(t)
	seedAdminFixture(t, store)
	cfg := withSessionKey(t, Config{Addr: ":0", DataDir: t.TempDir(), AuthMode: "token", Token: "operator-secret", AdminUserIDs: nil, Metrics: NewMetrics()}, store)
	token, err := issueSessionToken(cfg, "local:ordinary", "access", time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("issue session token: %v", err)
	}
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

// TestAdminUserSetPlanCreatesWhenMissing proves an operator can comp/create a
// subscription for a user who never subscribed: the set-plan call now creates a
// provider-neutral "manual" record instead of returning 412.
func TestAdminUserSetPlanCreatesWhenMissing(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	if err := store.UpsertUser(User{ID: "uns", Provider: "github", Subject: "n", Email: "n@e.com", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/uns/plan", adminToken, `{"plan":"annual","status":"active"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	sub, ok, err := store.GetSubscription("uns")
	if err != nil || !ok {
		t.Fatalf("GetSubscription: ok=%v err=%v", ok, err)
	}
	if sub.Plan != "annual" || sub.Status != "active" || sub.Provider != "manual" {
		t.Errorf("created subscription = %q/%q/%q, want annual/active/manual", sub.Plan, sub.Status, sub.Provider)
	}
	if !subscriptionCloudActive(sub, time.Now().UTC()) {
		t.Error("comped subscription should be entitlement-active")
	}
}

// TestAdminUserSetPlanRejectsBadStatus proves a free-text status that the
// entitlement seam wouldn't understand is refused rather than silently stored.
func TestAdminUserSetPlanRejectsBadStatus(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store) // ua: monthly/active
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/ua/plan", adminToken, `{"status":"vip"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	// The original status must be untouched.
	sub, ok, _ := store.GetSubscription("ua")
	if !ok || sub.Status != "active" {
		t.Errorf("status = %q, want unchanged active", sub.Status)
	}
}

// TestAdminUserSuspend proves suspend blocks the cloud entitlement and reinstate
// restores it, and that an admin cannot suspend their own account.
func TestAdminUserSuspend(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	seedAdminFixture(t, store) // ua active
	ctx := context.Background()
	cfg := Config{Billing: true}

	// Baseline: active → entitled.
	if active, _ := IsCloudActive(ctx, cfg, store, AuthUser{ID: "ua"}); !active {
		t.Fatal("ua should be entitled before suspension")
	}

	// Suspend.
	w := adminReq(t, mux, http.MethodPost, "/v1/admin/users/ua/suspend", adminToken, `{"suspended":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("suspend status = %d; body: %s", w.Code, w.Body.String())
	}
	if suspended, _ := store.IsUserSuspended("ua"); !suspended {
		t.Fatal("ua should be marked suspended")
	}
	if active, _ := IsCloudActive(ctx, cfg, store, AuthUser{ID: "ua"}); active {
		t.Fatal("suspended user must not be cloud-active")
	}

	// Reinstate.
	w = adminReq(t, mux, http.MethodPost, "/v1/admin/users/ua/suspend", adminToken, `{"suspended":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reinstate status = %d", w.Code)
	}
	if active, _ := IsCloudActive(ctx, cfg, store, AuthUser{ID: "ua"}); !active {
		t.Fatal("reinstated user should be entitled again")
	}

	// An admin cannot suspend themselves.
	adminID := resolvedAdminID(adminToken)
	if err := store.UpsertUser(User{ID: adminID, Provider: "token", Subject: "admin", Email: "a@e.com", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	w = adminReq(t, mux, http.MethodPost, "/v1/admin/users/"+adminID+"/suspend", adminToken, `{"suspended":true}`)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("self-suspend status = %d, want 412", w.Code)
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
