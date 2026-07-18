// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---- Config.IsAdmin --------------------------------------------------------

func TestConfigIsAdmin(t *testing.T) {
	cases := []struct {
		name      string
		ids       []string
		userID    string
		wantAdmin bool
	}{
		{"empty list denies everyone", nil, "u1", false},
		{"member of list allowed", []string{"u1", "u2"}, "u1", true},
		{"non-member denied", []string{"u1", "u2"}, "u3", false},
		{"empty userID denied", []string{"u1"}, "", false},
		{"whitespace userID denied", []string{"u1"}, "  ", false},
		{"exact match required", []string{"u1"}, "u1extra", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{AdminUserIDs: tc.ids}
			got := cfg.IsAdmin(tc.userID)
			if got != tc.wantAdmin {
				t.Fatalf("IsAdmin(%q) = %v, want %v", tc.userID, got, tc.wantAdmin)
			}
		})
	}
}

func TestFromEnvLoadsAdminUserIDs(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_ADMIN_USER_IDS", "u1, u2 , , u3")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if len(cfg.AdminUserIDs) != 3 || cfg.AdminUserIDs[0] != "u1" ||
		cfg.AdminUserIDs[1] != "u2" || cfg.AdminUserIDs[2] != "u3" {
		t.Fatalf("AdminUserIDs = %+v", cfg.AdminUserIDs)
	}
}

// ---- planMonthlyCents ------------------------------------------------------

func TestPlanMonthlyCents(t *testing.T) {
	cases := []struct {
		plan string
		want int64
	}{
		{"monthly", 999},
		{"MONTHLY", 999},
		{"annual", 825},
		{"Annual", 825},
		{"unknown", 0},
		{"", 0},
		{"enterprise", 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.plan, func(t *testing.T) {
			got := planMonthlyCents(tc.plan)
			if got != tc.want {
				t.Fatalf("planMonthlyCents(%q) = %d, want %d", tc.plan, got, tc.want)
			}
		})
	}
}

// ---- AdminOverview (aggregate correctness) ---------------------------------

func seedAdminFixture(t *testing.T, store *Store) {
	t.Helper()
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	users := []User{
		{ID: "ua", Provider: "github", Subject: "alice", Email: "alice@example.com", CreatedAt: now},
		{ID: "ub", Provider: "github", Subject: "bob", Email: "bob@example.com", CreatedAt: now},
		{ID: "uc", Provider: "google", Subject: "carol", Email: "carol@example.com", CreatedAt: now},
		{ID: "ud", Provider: "google", Subject: "dave", Email: "dave@example.com", CreatedAt: now},
	}
	for _, u := range users {
		if err := store.UpsertUser(u); err != nil {
			t.Fatalf("UpsertUser %s: %v", u.ID, err)
		}
	}
	subs := []Subscription{
		{UserID: "ua", ProviderCustomer: "cus_1", ProviderSubscription: "sub_1", Status: "active", Plan: "monthly", UpdatedAt: now},
		{UserID: "ub", ProviderCustomer: "cus_2", ProviderSubscription: "sub_2", Status: "trialing", Plan: "annual", UpdatedAt: now},
		{UserID: "uc", ProviderCustomer: "cus_3", ProviderSubscription: "sub_3", Status: "past_due", Plan: "monthly", UpdatedAt: now},
		{UserID: "ud", ProviderCustomer: "cus_4", ProviderSubscription: "sub_4", Status: "canceled", Plan: "monthly", UpdatedAt: now},
	}
	for _, sub := range subs {
		if err := store.PutSubscription(sub); err != nil {
			t.Fatalf("PutSubscription %s: %v", sub.UserID, err)
		}
	}
	// usage for today — use time.Now() so the handler's time.Now()-based query matches
	// on any calendar day (a hardcoded date would only match on the day it was written).
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if _, err := store.AddUsage("ua", today, 5, 100); err != nil {
		t.Fatalf("AddUsage ua: %v", err)
	}
	if _, err := store.AddUsage("ub", today, 3, 50); err != nil {
		t.Fatalf("AddUsage ub: %v", err)
	}
}

func TestAdminOverviewAggregates(t *testing.T) {
	store := openTestStore(t)
	seedAdminFixture(t, store)
	today := time.Now().UTC() // must match the day seeded by seedAdminFixture
	stats, err := store.AdminOverview(today)
	if err != nil {
		t.Fatalf("AdminOverview: %v", err)
	}
	if stats.TotalUsers != 4 {
		t.Errorf("TotalUsers = %d, want 4", stats.TotalUsers)
	}
	if stats.SubsActive != 1 {
		t.Errorf("SubsActive = %d, want 1", stats.SubsActive)
	}
	if stats.SubsTrialing != 1 {
		t.Errorf("SubsTrialing = %d, want 1", stats.SubsTrialing)
	}
	if stats.SubsPastDue != 1 {
		t.Errorf("SubsPastDue = %d, want 1", stats.SubsPastDue)
	}
	if stats.SubsCanceled != 1 {
		t.Errorf("SubsCanceled = %d, want 1", stats.SubsCanceled)
	}
	// active=monthly(999) + trialing=annual(825) = 1824
	wantMRR := int64(999 + 825)
	if stats.EstimatedMRRCents != wantMRR {
		t.Errorf("EstimatedMRRCents = %d, want %d", stats.EstimatedMRRCents, wantMRR)
	}
	if stats.TodayRequests != 8 {
		t.Errorf("TodayRequests = %d, want 8", stats.TodayRequests)
	}
	if stats.TodayTokens != 150 {
		t.Errorf("TodayTokens = %d, want 150", stats.TodayTokens)
	}
}

func TestAdminOverviewEmptyDB(t *testing.T) {
	store := openTestStore(t)
	stats, err := store.AdminOverview(time.Now().UTC())
	if err != nil {
		t.Fatalf("AdminOverview on empty db: %v", err)
	}
	if stats.TotalUsers != 0 || stats.TodayRequests != 0 || stats.TotalBlobBytes != 0 {
		t.Errorf("expected all-zeros on empty db, got %+v", stats)
	}
}

// ---- ListUsers (pagination + secret exclusion) -----------------------------

func TestListUsers(t *testing.T) {
	store := openTestStore(t)
	seedAdminFixture(t, store)

	// default limit
	rows, err := store.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("ListUsers len = %d, want 4", len(rows))
	}
	// cross-tenant: no secrets, no AI ciphertext, no blob bytes in the struct
	for _, row := range rows {
		if row.ID == "" {
			t.Error("row.ID is empty")
		}
		if row.Email == "" {
			t.Error("row.Email is empty")
		}
		if row.Provider == "" {
			t.Error("row.Provider is empty")
		}
	}

	// pagination: offset
	page1, err := store.ListUsers(2, 0)
	if err != nil {
		t.Fatalf("ListUsers page1: %v", err)
	}
	page2, err := store.ListUsers(2, 2)
	if err != nil {
		t.Fatalf("ListUsers page2: %v", err)
	}
	if len(page1) != 2 || len(page2) != 2 {
		t.Fatalf("pagination: page1=%d page2=%d, want 2/2", len(page1), len(page2))
	}
	// no duplicates between pages
	ids1 := map[string]bool{page1[0].ID: true, page1[1].ID: true}
	if ids1[page2[0].ID] || ids1[page2[1].ID] {
		t.Error("pagination returned duplicate rows across pages")
	}

	// limit cap at 200
	capped, err := store.ListUsers(9999, 0)
	if err != nil {
		t.Fatalf("ListUsers capped: %v", err)
	}
	if len(capped) != 4 { // only 4 users in fixture
		t.Fatalf("capped ListUsers len = %d, want 4", len(capped))
	}
}

func TestListUsersFilteredByEmail(t *testing.T) {
	store := openTestStore(t)
	seedAdminFixture(t, store) // alice/bob/carol/dave @example.com

	// Case-insensitive substring match on email.
	rows, err := store.ListUsersFiltered(50, 0, "ALICE")
	if err != nil {
		t.Fatalf("ListUsersFiltered: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != "alice@example.com" {
		t.Fatalf("search alice = %+v, want just alice", rows)
	}
	// A shared substring matches every seeded user.
	rows, err = store.ListUsersFiltered(50, 0, "example.com")
	if err != nil {
		t.Fatalf("ListUsersFiltered: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("search example.com len = %d, want 4", len(rows))
	}
	// LIKE wildcards in the term are treated literally (no user contains a literal
	// '%', so this must return nothing rather than matching everyone).
	rows, err = store.ListUsersFiltered(50, 0, "%")
	if err != nil {
		t.Fatalf("ListUsersFiltered wildcard: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("literal %% search len = %d, want 0", len(rows))
	}
}

// TestAdminUsersPaginationHasMore proves the /v1/admin/users endpoint reports
// hasMore correctly using its page+1 probe: a full first page signals more, the
// final partial page does not.
func TestAdminUsersPaginationHasMore(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("u%02d", i)
		if err := store.UpsertUser(User{
			ID: id, Provider: "github", Subject: id,
			Email: fmt.Sprintf("user%02d@example.com", i), CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	decode := func(body string) AdminUsersResponse {
		var r AdminUsersResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return r
	}

	// Page 1: 25 rows, more to come.
	w := adminReq(t, mux, http.MethodGet, "/v1/admin/users?limit=25&offset=0", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("page1 status = %d", w.Code)
	}
	p1 := decode(w.Body.String())
	if len(p1.Users) != 25 || !p1.HasMore {
		t.Fatalf("page1: len=%d hasMore=%v, want 25/true", len(p1.Users), p1.HasMore)
	}
	// Page 2: remaining 5 rows, no more.
	w = adminReq(t, mux, http.MethodGet, "/v1/admin/users?limit=25&offset=25", adminToken, "")
	p2 := decode(w.Body.String())
	if len(p2.Users) != 5 || p2.HasMore {
		t.Fatalf("page2: len=%d hasMore=%v, want 5/false", len(p2.Users), p2.HasMore)
	}
	// Search narrows and disables paging.
	w = adminReq(t, mux, http.MethodGet, "/v1/admin/users?limit=25&offset=0&q=user07", adminToken, "")
	ps := decode(w.Body.String())
	if len(ps.Users) != 1 || ps.HasMore || ps.Users[0].Email != "user07@example.com" {
		t.Fatalf("search: len=%d hasMore=%v rows=%+v", len(ps.Users), ps.HasMore, ps.Users)
	}
}

func TestListUsersSubscriptionStatus(t *testing.T) {
	store := openTestStore(t)
	seedAdminFixture(t, store)
	rows, err := store.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	// find alice (ua) — active/monthly
	var alice AdminUserRow
	for _, r := range rows {
		if r.ID == "ua" {
			alice = r
		}
	}
	if alice.SubscriptionStatus != "active" {
		t.Errorf("alice status = %q, want active", alice.SubscriptionStatus)
	}
	if alice.SubscriptionPlan != "monthly" {
		t.Errorf("alice plan = %q, want monthly", alice.SubscriptionPlan)
	}
}

func TestListUsersNoSubscription(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "ux", Provider: "github", Subject: "x", Email: "x@x.com", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	rows, err := store.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	if rows[0].SubscriptionStatus != "" {
		t.Errorf("no-subscription status = %q, want empty", rows[0].SubscriptionStatus)
	}
}

// ---- HTTP endpoints (authz + response shape) --------------------------------

func newAdminTestMux(t *testing.T, adminID string) (http.Handler, *Store) {
	t.Helper()
	store := openTestStore(t)
	cfg := Config{
		Addr:         ":0",
		DataDir:      t.TempDir(),
		AuthMode:     "token",
		Token:        "admin-secret",
		AdminUserIDs: []string{adminID},
		Metrics:      NewMetrics(),
	}
	return NewMux(cfg, store), store
}

func adminBearer(token string) string { return "Bearer " + token }

// authUserForToken resolves the token to a user ID via the same path the
// production code takes (sha256 hash → "token:<hex24>").
// We can also just look up the ID by calling authUserFromToken.
func resolvedAdminID(token string) string {
	return authUserFromToken(token).ID
}

func TestHandleAdminOverviewUnauthenticated(t *testing.T) {
	mux, _ := newAdminTestMux(t, resolvedAdminID("admin-secret"))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/overview", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandleAdminOverviewNonAdmin(t *testing.T) {
	// Token authenticates fine, but AdminUserIDs is empty so nobody is admin.
	token := "admin-secret"
	store := openTestStore(t)
	cfg := Config{
		Addr:         ":0",
		DataDir:      t.TempDir(),
		AuthMode:     "token",
		Token:        token,
		AdminUserIDs: nil, // empty — deny by default
		Metrics:      NewMetrics(),
	}
	mux := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/overview", nil)
	req.Header.Set("Authorization", adminBearer(token))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestHandleAdminOverviewAdmin(t *testing.T) {
	adminToken := "admin-secret"
	adminID := resolvedAdminID(adminToken)
	mux, store := newAdminTestMux(t, adminID)
	// seed some data so overview is non-trivial
	now := time.Now().UTC()
	_ = store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "s1", Email: "e@e.com", CreatedAt: now})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/overview", nil)
	req.Header.Set("Authorization", adminBearer(adminToken))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminOverviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TotalUsers < 1 {
		t.Errorf("TotalUsers = %d, want ≥1", resp.TotalUsers)
	}
	if resp.Day == "" {
		t.Error("Day field is empty")
	}
}

func TestHandleAdminUsersUnauthenticated(t *testing.T) {
	mux, _ := newAdminTestMux(t, resolvedAdminID("admin-secret"))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandleAdminUsersNonAdmin(t *testing.T) {
	// Token authenticates fine, but AdminUserIDs is empty so nobody is admin.
	token := "admin-secret"
	store := openTestStore(t)
	cfg := Config{
		Addr:         ":0",
		DataDir:      t.TempDir(),
		AuthMode:     "token",
		Token:        token,
		AdminUserIDs: nil, // empty — deny by default
		Metrics:      NewMetrics(),
	}
	mux := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Authorization", adminBearer(token))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestHandleAdminUsersAdmin(t *testing.T) {
	adminToken := "admin-secret"
	adminID := resolvedAdminID(adminToken)
	mux, store := newAdminTestMux(t, adminID)
	seedAdminFixture(t, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users?limit=2&offset=0", nil)
	req.Header.Set("Authorization", adminBearer(adminToken))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminUsersResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Users) != 2 {
		t.Errorf("len(Users) = %d, want 2", len(resp.Users))
	}
	if resp.Limit != 2 {
		t.Errorf("Limit = %d, want 2", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("Offset = %d, want 0", resp.Offset)
	}
	// verify no secrets fields are present in the JSON
	raw := w.Body.Bytes()
	for _, forbidden := range []string{"ciphertext", "nonce", "dataset_json", "dataset"} {
		if containsBytes(raw, forbidden) {
			t.Errorf("response JSON contains forbidden field %q", forbidden)
		}
	}
}

func TestHandleAdminUsersLimitCap(t *testing.T) {
	adminToken := "cap-secret"
	adminID := resolvedAdminID(adminToken)
	store := openTestStore(t)
	cfg := Config{
		Addr:         ":0",
		DataDir:      t.TempDir(),
		AuthMode:     "token",
		Token:        adminToken,
		AdminUserIDs: []string{adminID},
		Metrics:      NewMetrics(),
	}
	mux := NewMux(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users?limit=999", nil)
	req.Header.Set("Authorization", adminBearer(adminToken))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp AdminUsersResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Limit != 200 {
		t.Errorf("Limit = %d, want 200 (capped)", resp.Limit)
	}
}

func TestAdminCrossTenantSafety(t *testing.T) {
	// Confirm that the users list response contains no AI keys, blob bytes, or
	// snapshot datasets — cross-tenant secrets stay server-side.
	adminToken := "xten-secret"
	adminID := resolvedAdminID(adminToken)
	store := openTestStore(t)
	now := time.Now().UTC()
	_ = store.UpsertUser(User{ID: "v1", Provider: "github", Subject: "v1", Email: "v1@v.com", CreatedAt: now})

	cfg := Config{
		Addr:         ":0",
		DataDir:      t.TempDir(),
		AuthMode:     "token",
		Token:        adminToken,
		AdminUserIDs: []string{adminID},
		MasterKey:    "12345678901234567890123456789012",
		Metrics:      NewMetrics(),
	}
	if err := store.PutAIKey("v1", "openai", "sk-secret-key", []byte(cfg.MasterKey)); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	mux := NewMux(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.Header.Set("Authorization", adminBearer(adminToken))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	raw := w.Body.Bytes()
	// The AI key plaintext and ciphertext must never appear in the response.
	for _, forbidden := range []string{"sk-secret-key", "ciphertext", "nonce"} {
		if containsBytes(raw, forbidden) {
			t.Errorf("admin users response leaked %q", forbidden)
		}
	}
}

func containsBytes(haystack []byte, needle string) bool {
	return len(needle) > 0 && string(haystack) != "" && func() bool {
		h := string(haystack)
		for i := 0; i <= len(h)-len(needle); i++ {
			if h[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	}()
}
