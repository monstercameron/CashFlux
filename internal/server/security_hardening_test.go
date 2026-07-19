// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
)

// --- Fix #1: rate-limiter bucket eviction (unbounded-map DoS) -----------------

// TestFixedWindowLimiterEvictsStaleBuckets proves the limiter reclaims buckets whose
// window has elapsed, so a flood of distinct keys can't pin memory for the process
// lifetime. Without the sweep the map only ever grew.
func TestFixedWindowLimiterEvictsStaleBuckets(t *testing.T) {
	limiter := newFixedWindowLimiter(5)
	base := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	// Seed 1000 distinct keys at t0 (simulating 1000 spoofed/rotated source IPs).
	for i := 0; i < 1000; i++ {
		limiter.allow(net.IPv4(10, 0, byte(i/256), byte(i%256)).String(), base)
	}
	if got := len(limiter.buckets); got != 1000 {
		t.Fatalf("after seeding, bucket count = %d, want 1000", got)
	}
	// Advance past the window and touch a single fresh key. The sweep must run and
	// drop every stale bucket, leaving only the just-touched key.
	later := base.Add(2 * time.Minute)
	limiter.allow("203.0.113.9", later)
	if got := len(limiter.buckets); got != 1 {
		t.Fatalf("after sweep, bucket count = %d, want 1 (stale buckets evicted)", got)
	}
}

// TestFixedWindowLimiterSweepKeepsActiveBuckets makes sure eviction is not overzealous:
// a key still inside its current window must survive a sweep so limiting stays correct.
func TestFixedWindowLimiterSweepKeepsActiveBuckets(t *testing.T) {
	limiter := newFixedWindowLimiter(2)
	base := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	limiter.allow("198.51.100.1", base)           // window opens at base
	limiter.allow("198.51.100.2", base.Add(90*time.Second)) // triggers a sweep at +90s
	// 198.51.100.1's window (base) is now >1min old → evicted. 198.51.100.2 is fresh.
	if _, ok := limiter.buckets["198.51.100.2"]; !ok {
		t.Fatalf("active bucket was evicted")
	}
	// A brand-new key at the same instant must get a fresh window (count resets), so it
	// is allowed up to the limit rather than inheriting a stale count.
	if !limiter.allow("198.51.100.1", base.Add(90*time.Second)) {
		t.Fatalf("re-seen key after its window elapsed was not allowed a fresh window")
	}
}

// --- Fix #1: trusted-proxy-aware client IP resolution -------------------------

func TestRateLimitClientIPIgnoresUntrustedForwardingHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "198.51.100.5:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	req.Header.Set("X-Real-IP", "203.0.113.99")
	// No trusted proxies configured → headers are attacker-controlled → use the peer.
	if got := rateLimitClientIP(req, Config{}); got != "198.51.100.5" {
		t.Fatalf("untrusted client ip = %q, want the socket peer 198.51.100.5", got)
	}
}

func TestRateLimitClientIPHonorsTrustedProxyChain(t *testing.T) {
	_, proxyNet, _ := net.ParseCIDR("10.0.0.0/8")
	cfg := Config{TrustedProxies: []*net.IPNet{proxyNet}}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.1.1.1:443" // trusted proxy peer
	// XFF: real client, then two internal trusted hops. Resolver must return the
	// rightmost NON-trusted entry (the real client as seen by our edge).
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.9.9.9, 10.1.1.1")
	if got := rateLimitClientIP(req, cfg); got != "203.0.113.7" {
		t.Fatalf("trusted-proxy client ip = %q, want 203.0.113.7", got)
	}
}

func TestRateLimitClientIPUntrustedPeerIgnoresForgedChain(t *testing.T) {
	_, proxyNet, _ := net.ParseCIDR("10.0.0.0/8")
	cfg := Config{TrustedProxies: []*net.IPNet{proxyNet}}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "203.0.113.200:443" // NOT a trusted proxy
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // forged
	if got := rateLimitClientIP(req, cfg); got != "203.0.113.200" {
		t.Fatalf("untrusted peer ip = %q, want the real peer 203.0.113.200 (forged XFF ignored)", got)
	}
}

// --- Fix #1: trusted-proxy config parsing -------------------------------------

func TestParseTrustedProxies(t *testing.T) {
	nets, err := parseTrustedProxies(" 10.0.0.0/8 , 203.0.113.9 , ::1 ")
	if err != nil {
		t.Fatalf("parseTrustedProxies: %v", err)
	}
	if len(nets) != 3 {
		t.Fatalf("parsed %d networks, want 3", len(nets))
	}
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.255.1.1", true},   // inside 10.0.0.0/8
		{"11.0.0.1", false},    // outside
		{"203.0.113.9", true},  // bare IP → /32
		{"203.0.113.10", false},
		{"::1", true}, // bare IPv6 → /128
	}
	for _, tc := range cases {
		if got := ipInNets(tc.ip, nets); got != tc.want {
			t.Fatalf("ipInNets(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestParseTrustedProxiesRejectsGarbage(t *testing.T) {
	if _, err := parseTrustedProxies("10.0.0.0/8, not-an-ip"); err == nil {
		t.Fatalf("expected error for an invalid trusted-proxy entry, got nil")
	}
}

func TestFromEnvRejectsInvalidTrustedProxies(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_TRUSTED_PROXIES", "300.300.300.300")
	if _, err := FromEnv(); err == nil {
		t.Fatalf("FromEnv accepted an invalid trusted proxy; want a hard error")
	}
}

// --- Fix #2: webhook check-apply-record atomicity -----------------------------

// TestStripeWebhookFailedApplyIsNotDeduped is the regression test for the billing
// integrity bug: a webhook whose apply FAILS must NOT be recorded as seen, so the
// provider's retry re-applies it. Previously the event id was recorded before apply,
// so one transient/out-of-order failure permanently swallowed the state change.
func TestStripeWebhookFailedApplyIsNotDeduped(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 14, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	cfg := Config{AuthMode: "token", Billing: true, StripeWebhookSecret: "whsec_test"}
	h := NewMux(cfg, store)
	send := func(payload []byte) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/stripe/webhook", bytes.NewReader(payload))
		req.Header.Set(stripeSignatureHeader, testStripeSignature(t, payload, cfg.StripeWebhookSecret, time.Now().UTC()))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// payment_failed for a subscription the server has never seen → apply fails.
	failing := []byte(`{"id":"evt_pf","type":"invoice.payment_failed","data":{"object":{"customer":"cus_x","subscription":"sub_x"}}}`)
	if code := send(failing); code != http.StatusBadRequest {
		t.Fatalf("first (failing) webhook status = %d, want 400", code)
	}
	// The failed event must NOT have been recorded — otherwise the retry is deduped away.
	if seen, err := store.HasWebhookEvent("stripe", "evt_pf"); err != nil || seen {
		t.Fatalf("failed event was recorded (seen=%v err=%v); it must be retryable", seen, err)
	}

	// The subscription now exists (e.g. the out-of-order created event finally arrives).
	if err := store.PutSubscription(Subscription{
		UserID: "u1", Provider: "stripe", ProviderCustomer: "cus_x", ProviderSubscription: "sub_x",
		Status: "active", Plan: "personal_monthly", CurrentPeriodEnd: now.AddDate(0, 0, 20), UpdatedAt: now,
	}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}

	// Retrying the SAME event id must NOT be deduped — it re-applies and lands past_due.
	if code := send(failing); code != http.StatusNoContent {
		t.Fatalf("retry webhook status = %d, want 204 (re-applied, not deduped)", code)
	}
	got, ok, err := store.GetSubscription("u1")
	if err != nil || !ok {
		t.Fatalf("GetSubscription = %v/%v", ok, err)
	}
	if got.Status != "past_due" {
		t.Fatalf("status = %q, want past_due (retry applied)", got.Status)
	}
	if seen, err := store.HasWebhookEvent("stripe", "evt_pf"); err != nil || !seen {
		t.Fatalf("after successful apply the event must be recorded (seen=%v err=%v)", seen, err)
	}
}

// TestStripeWebhookSuccessfulEventIsDedupedOnReplay is the complementary guarantee:
// once an event applies successfully it IS recorded, so a genuine replay is a no-op
// and cannot overwrite newer state. (Pairs with the failed-apply test above.)
func TestStripeWebhookSuccessfulEventIsDedupedOnReplay(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 14, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	cfg := Config{AuthMode: "token", Billing: true, StripeWebhookSecret: "whsec_test"}
	h := NewMux(cfg, store)
	send := func(payload []byte) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/stripe/webhook", bytes.NewReader(payload))
		req.Header.Set(stripeSignatureHeader, testStripeSignature(t, payload, cfg.StripeWebhookSecret, time.Now().UTC()))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	active := []byte(`{"id":"evt_ok","type":"customer.subscription.updated","data":{"object":{"id":"sub_1","customer":"cus_1","status":"active","metadata":{"user_id":"u1","plan":"personal_annual"}}}}`)
	if code := send(active); code != http.StatusNoContent {
		t.Fatalf("first webhook = %d, want 204", code)
	}
	// Replay the same id carrying a canceled status — must be ignored.
	replay := []byte(`{"id":"evt_ok","type":"customer.subscription.updated","data":{"object":{"id":"sub_1","customer":"cus_1","status":"canceled","metadata":{"user_id":"u1","plan":"personal_annual"}}}}`)
	if code := send(replay); code != http.StatusNoContent {
		t.Fatalf("replay webhook = %d, want 204", code)
	}
	got, _, err := store.GetSubscription("u1")
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("replay overwrote state: status = %q, want active", got.Status)
	}
}

// TestStripeWebhookConcurrentDuplicatesApplyExactlyOnce fires many simultaneous
// deliveries of the SAME event id and asserts the effect (a billing signup metric)
// lands exactly once. This exercises the webhook lock: without serialized
// check-apply-record, concurrent deliveries could all slip past the dedupe check and
// double-apply (double-counting revenue events / re-issuing entitlement changes).
func TestStripeWebhookConcurrentDuplicatesApplyExactlyOnce(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 14, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	metrics := NewMetrics()
	cfg := Config{AuthMode: "token", Billing: true, StripeWebhookSecret: "whsec_test", Metrics: metrics}
	h := NewMux(cfg, store)
	payload := []byte(`{"id":"evt_cc","type":"checkout.session.completed","data":{"object":{"customer":"cus_1","subscription":"sub_1","client_reference_id":"u1","metadata":{"plan":"personal_annual","status":"trialing"}}}}`)
	sig := testStripeSignature(t, payload, cfg.StripeWebhookSecret, time.Now().UTC())

	const n = 24
	var wg sync.WaitGroup
	codes := make([]int, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/billing/stripe/webhook", bytes.NewReader(payload))
			req.Header.Set(stripeSignatureHeader, sig)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			codes[i] = rr.Code
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusNoContent {
			t.Fatalf("concurrent delivery %d status = %d, want 204", i, code)
		}
	}
	var out strings.Builder
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), `cashflux_billing_events_total{event="signup",plan="personal_annual",status="trialing"} 1`) {
		t.Fatalf("signup metric did not land exactly once under concurrency:\n%s", out.String())
	}
}

// TestHasWebhookEvent covers the store read half of the atomic flow directly.
func TestHasWebhookEvent(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 14, 0, 0, 0, time.UTC)
	if seen, err := store.HasWebhookEvent("stripe", "evt_1"); err != nil || seen {
		t.Fatalf("unseen event reported seen=%v err=%v", seen, err)
	}
	// Empty ids are never "seen" (they can't be deduped and always apply).
	if seen, err := store.HasWebhookEvent("stripe", ""); err != nil || seen {
		t.Fatalf("empty event id reported seen=%v err=%v", seen, err)
	}
	if _, err := store.RecordWebhookEventOnce("stripe", "evt_1", now); err != nil {
		t.Fatalf("RecordWebhookEventOnce: %v", err)
	}
	if seen, err := store.HasWebhookEvent("stripe", "evt_1"); err != nil || !seen {
		t.Fatalf("recorded event reported seen=%v err=%v, want seen", seen, err)
	}
}

// --- Fix #3: wildcard CORS never rides credentials ----------------------------

// TestWriteCORSWildcardWithholdsCredentials proves all-origins mode emits a literal
// "*" and NEVER Access-Control-Allow-Credentials — so no site can drive credentialed
// (cookie-bearing) cross-origin requests as a victim.
func TestWriteCORSWildcardWithholdsCredentials(t *testing.T) {
	cfg := Config{AppOrigin: "*"}
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	if !writeCORS(rr, req, cfg) {
		t.Fatalf("wildcard origin was rejected")
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want literal * (never a reflected origin) in wildcard mode", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Allow-Credentials = %q, want empty in wildcard mode", got)
	}
}

// TestWriteCORSNamedOriginReflectsWithCredentials confirms the intended single-origin
// deployment still works: the one configured origin is reflected WITH credentials, and
// any other origin is denied outright.
func TestWriteCORSNamedOriginReflectsWithCredentials(t *testing.T) {
	cfg := Config{AppOrigin: "https://app.example.com"}

	ok := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	ok.Header.Set("Origin", "https://app.example.com")
	okRR := httptest.NewRecorder()
	if !writeCORS(okRR, ok, cfg) {
		t.Fatalf("configured origin was rejected")
	}
	if got := okRR.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Allow-Origin = %q, want the reflected configured origin", got)
	}
	if got := okRR.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true for the configured origin", got)
	}

	bad := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	bad.Header.Set("Origin", "https://evil.example.com")
	badRR := httptest.NewRecorder()
	if writeCORS(badRR, bad, cfg) {
		t.Fatalf("a non-configured origin was allowed; want denial")
	}
	if got := badRR.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty for a denied origin", got)
	}
}

// --- Right-to-erasure: DeleteAccount must not depend on FK cascade ------------

// TestDeleteAccountPurgesEverythingWithoutCascade seeds a fully-populated account,
// turns foreign_keys OFF (simulating a pooled connection that lost the per-connection
// pragma), deletes the account, and asserts every child table is emptied. This proves
// erasure is self-contained — it no longer relies on ON DELETE CASCADE, so a deleted
// user's encrypted AI keys, snapshots, and refresh tokens can't be silently orphaned
// (a surviving refresh token could otherwise resurrect the account).
func TestDeleteAccountPurgesEverythingWithoutCascade(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	master := []byte("0123456789abcdef0123456789abcdef")
	const uid = "u-del"
	if err := store.UpsertUser(User{ID: uid, Provider: "github", Subject: "del", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutWorkspace(Workspace{ID: "ws1", UserID: uid, Name: "Home", Version: 1, UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "ws1", Dataset: []byte(`{"a":1}`), Version: 1, UpdatedAt: now}, 1<<20, 5); err != nil {
		t.Fatalf("PutSnapshot: %v", err)
	}
	// A second snapshot version to populate snapshot_history too.
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "ws1", Dataset: []byte(`{"a":2}`), Version: 2, UpdatedAt: now.Add(time.Minute)}, 1<<20, 5); err != nil {
		t.Fatalf("PutSnapshot v2: %v", err)
	}
	if err := store.PutAIKey(uid, "openai", "sk-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	if _, err := store.AddUsage(uid, now, 3, 100); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	if err := store.PutRefreshSession(RefreshSession{JTI: "jti1", FamilyID: "fam1", UserID: uid, TokenHash: "hash1", ExpiresAt: now.Add(24 * time.Hour)}); err != nil {
		t.Fatalf("PutRefreshSession: %v", err)
	}
	if err := store.PutIdempotencyResult(IdempotencyResult{UserID: uid, Route: "/v1/billing/checkout", Key: "k1", RequestHash: "rh", ResponseBody: []byte("{}"), CreatedAt: now}); err != nil {
		t.Fatalf("PutIdempotencyResult: %v", err)
	}
	if err := store.PutSubscription(Subscription{UserID: uid, Provider: "stripe", ProviderCustomer: "cus", ProviderSubscription: "sub", Status: "active", Plan: "personal_monthly", UpdatedAt: now}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}
	// An audit event MUST survive deletion (append-only, tamper-evident, no FK).
	if _, err := store.AppendAuditEvent(AuditEvent{Timestamp: now, ActorID: uid, Action: "account.delete", TargetType: "user", TargetID: uid}); err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	// Simulate the degraded connection: cascade is now OFF.
	if _, err := store.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}

	deleted, err := store.DeleteAccount(uid)
	if err != nil || !deleted {
		t.Fatalf("DeleteAccount = %v/%v", deleted, err)
	}

	count := func(query string, args ...any) int {
		var n int
		if err := store.db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("count query %q: %v", query, err)
		}
		return n
	}
	checks := []struct {
		name  string
		query string
	}{
		{"users", `SELECT COUNT(*) FROM users WHERE id = ?`},
		{"workspaces", `SELECT COUNT(*) FROM workspaces WHERE user_id = ?`},
		{"snapshots", `SELECT COUNT(*) FROM snapshots WHERE workspace_id = 'ws1'`},
		{"snapshot_history", `SELECT COUNT(*) FROM snapshot_history WHERE workspace_id = 'ws1'`},
		{"ai_keys", `SELECT COUNT(*) FROM ai_keys WHERE user_id = ?`},
		{"usage", `SELECT COUNT(*) FROM usage WHERE user_id = ?`},
		{"refresh_tokens", `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ?`},
		{"idempotency_keys", `SELECT COUNT(*) FROM idempotency_keys WHERE user_id = ?`},
		{"subscriptions", `SELECT COUNT(*) FROM subscriptions WHERE user_id = ?`},
	}
	for _, c := range checks {
		var n int
		if strings.Contains(c.query, "ws1") {
			n = count(c.query)
		} else {
			n = count(c.query, uid)
		}
		if n != 0 {
			t.Fatalf("%s still has %d row(s) after erasure (cascade OFF)", c.name, n)
		}
	}
	if got := count(`SELECT COUNT(*) FROM audit_events WHERE actor_id = ?`, uid); got == 0 {
		t.Fatalf("audit events were deleted; the append-only log must be retained")
	}
}

// --- Tenant isolation: snapshot read is scoped in SQL, not just by call order -

func TestGetSnapshotForUserIsolatesTenants(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	for _, u := range []string{"a", "b"} {
		if err := store.UpsertUser(User{ID: u, Provider: "token", Subject: u, CreatedAt: now}); err != nil {
			t.Fatalf("UpsertUser %s: %v", u, err)
		}
	}
	if err := store.PutWorkspace(Workspace{ID: "wsA", UserID: "a", Name: "A", Version: 1, UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "wsA", Dataset: []byte(`{"secret":true}`), Version: 1, UpdatedAt: now}, 1<<20, 5); err != nil {
		t.Fatalf("PutSnapshot: %v", err)
	}
	// Owner can read it.
	if _, ok, err := store.GetSnapshotForUser("a", "wsA"); err != nil || !ok {
		t.Fatalf("owner GetSnapshotForUser = %v/%v, want found", ok, err)
	}
	// A different tenant asking for the same workspace id gets nothing.
	if _, ok, err := store.GetSnapshotForUser("b", "wsA"); err != nil || ok {
		t.Fatalf("cross-tenant GetSnapshotForUser = %v/%v, want not found", ok, err)
	}
}

// --- AI daily cap: atomic reservation holds under concurrency -----------------

// TestAIDailyRequestCapHoldsUnderConcurrency fires many simultaneous chat requests
// against a cap of 5 and asserts EXACTLY 5 succeed — the atomic reservation closes the
// check-then-increment race that previously let concurrent callers overshoot the cap.
func TestAIDailyRequestCapHoldsUnderConcurrency(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	day := time.Date(2026, time.July, 18, 18, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:      master,
		RequestsPerDay: 5,
		Now:            func() time.Time { return day },
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":1}}`)),
			}, nil
		}),
	})
	const n = 25
	var ok, exhausted int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
				Model:    "gpt-5.4-mini",
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
			})
			if err == nil {
				atomic.AddInt64(&ok, 1)
			} else if strings.Contains(err.Error(), "daily ai request limit reached") {
				atomic.AddInt64(&exhausted, 1)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 5 {
		t.Fatalf("succeeded = %d, want exactly 5 (cap not overshot or undershot)", ok)
	}
	if exhausted != n-5 {
		t.Fatalf("rate-limited = %d, want %d", exhausted, n-5)
	}
	usage, _, err := store.GetUsage("u1", day)
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if usage.Requests != 5 {
		t.Fatalf("recorded requests = %d, want exactly 5", usage.Requests)
	}
}

// TestAIFailedRequestReleasesReservation proves a reserved slot is returned when the
// request fails upstream, so a failed attempt doesn't permanently consume quota.
func TestAIFailedRequestReleasesReservation(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	day := time.Date(2026, time.July, 18, 18, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	req := AIChatRequest{Model: "gpt-5.4-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}}

	// Cap of 1. First attempt fails upstream (500) → its reservation must be released.
	failing := NewAIService(store, AIServiceConfig{
		MasterKey: master, RequestsPerDay: 1, Now: func() time.Time { return day },
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"boom"}}`))}, nil
		}),
	})
	if _, err := failing.Chat(ctx, req); err == nil {
		t.Fatalf("expected an error from the 500 upstream")
	}
	if usage, ok, _ := store.GetUsage("u1", day); ok && usage.Requests != 0 {
		t.Fatalf("failed request left requests = %d, want 0 (slot released)", usage.Requests)
	}

	// A subsequent good request must therefore still be allowed under the cap of 1.
	working := NewAIService(store, AIServiceConfig{
		MasterKey: master, RequestsPerDay: 1, Now: func() time.Time { return day },
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":2}}`))}, nil
		}),
	})
	if _, err := working.Chat(ctx, req); err != nil {
		t.Fatalf("good request after a released failure was rejected: %v", err)
	}
	if usage, _, _ := store.GetUsage("u1", day); usage.Requests != 1 {
		t.Fatalf("recorded requests = %d, want 1", usage.Requests)
	}
}

// --- CSRF double-submit (constant-time compare) -------------------------------

func TestValidCSRF(t *testing.T) {
	build := func(cookie, header string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: sessionCSRFCookie, Value: cookie})
		}
		if header != "" {
			r.Header.Set(sessionCSRFHeader, header)
		}
		return r
	}
	if !validCSRF(build("tok-abc", "tok-abc")) {
		t.Fatalf("matching cookie+header was rejected")
	}
	if validCSRF(build("tok-abc", "tok-xyz")) {
		t.Fatalf("mismatched token was accepted")
	}
	if validCSRF(build("tok-abc", "")) {
		t.Fatalf("missing header was accepted")
	}
	if validCSRF(build("", "tok-abc")) {
		t.Fatalf("missing cookie was accepted")
	}
}

// TestOAuthHTTPClientHasTimeout locks in that outbound OAuth calls are bounded (they
// used the timeout-less http.DefaultClient before).
func TestOAuthHTTPClientHasTimeout(t *testing.T) {
	if oauthHTTPClient.Timeout <= 0 {
		t.Fatalf("oauthHTTPClient has no timeout; a hung provider could pin a worker")
	}
}

// TestSanitizeRequestIDStripsControlChars proves the client-supplied request id is
// cleaned at the source: control characters (newlines/CR/escapes/DEL) are dropped so
// it can't smuggle a forged log line or an injected response-header byte, and the
// length cap still holds.
func TestSanitizeRequestIDStripsControlChars(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc123", "abc123"},
		{"  spaced  ", "spaced"},
		{"line1\ninjected fake=log", "line1injected fake=log"},
		{"tab\there\r\n", "tabhere"},
		{"del\x7fbyte", "delbyte"},
		{"\n\r\x00", ""}, // control-only collapses to empty (upstream mints a fresh id)
	}
	for _, c := range cases {
		if got := sanitizeRequestID(c.in); got != c.want {
			t.Fatalf("sanitizeRequestID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := sanitizeRequestID(strings.Repeat("a", 200)); len(got) != 128 {
		t.Fatalf("length cap = %d, want 128", len(got))
	}
}
