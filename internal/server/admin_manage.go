// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// This file adds the admin *management* surface on top of the read-only overview/users
// list in admin.go: a single-user detail view, per-user usage analytics, and the
// account actions (set plan, revoke sessions, delete) an operator needs to actually
// maintain the business from the console. Every handler is admin-gated and audited,
// mirroring the guard pattern in admin.go. No secrets, AI ciphertext, or blob bytes are
// ever returned.

// AdminUserDetailResponse is the JSON body for GET /v1/admin/users/{id}: the user's
// profile plus subscription, storage, workspace count, and today's usage — a support
// agent's at-a-glance account summary.
type AdminUserDetailResponse struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Email              string `json:"email"`
	CreatedAt          string `json:"createdAt"`
	SubscriptionPlan   string `json:"subscriptionPlan,omitempty"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
	CurrentPeriodEnd   string `json:"currentPeriodEnd,omitempty"`
	TrialEnd           string `json:"trialEnd,omitempty"`
	WorkspaceCount     int    `json:"workspaceCount"`
	BlobBytes          int64  `json:"blobBytes"`
	UsageTodayRequests int64  `json:"usageTodayRequests"`
	UsageTodayTokens   int64  `json:"usageTodayTokens"`
}

// AdminUsageHistoryResponse is the JSON body for GET /v1/admin/users/{id}/usage: a
// newest-first window of daily usage rows for charting per-user activity.
type AdminUsageHistoryResponse struct {
	UserID string  `json:"userId"`
	Days   int     `json:"days"`
	Usage  []Usage `json:"usage"`
}

// AdminActionResponse is the uniform JSON body for a management action.
type AdminActionResponse struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	UserID string `json:"userId"`
	Detail string `json:"detail,omitempty"`
}

// AdminSetPlanRequest is the body of POST /v1/admin/users/{id}/plan — an operator
// override of the plan/status on an existing subscription (Stripe stays the source of
// truth for billing; this is a manual correction, e.g. comp or status fix).
type AdminSetPlanRequest struct {
	Plan   string `json:"plan"`
	Status string `json:"status"`
}

// adminGuard runs the shared admin gate (CORS, store, bearer, IsAdmin) and resolves the
// target user id from the {id} path value. On failure it has already written the error
// response and returns ok=false. action/resource label the audit entry on denial.
func adminGuard(cfg Config, store *Store, w http.ResponseWriter, r *http.Request, action, resource string) (AuthUser, string, bool) {
	if !writeCORS(w, r, cfg) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
		return AuthUser{}, "", false
	}
	if store == nil {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
		return AuthUser{}, "", false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
		return AuthUser{}, "", false
	}
	if !cfg.IsAdmin(user.ID) {
		auditFromRequest(r, store, user, action+".denied", "admin", resource)
		writeErrorJSON(w, ErrorReasonPermissionDenied, "admin access required")
		return AuthUser{}, "", false
	}
	target := strings.TrimSpace(r.PathValue("id"))
	if target == "" {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "user id is required")
		return AuthUser{}, "", false
	}
	return user, target, true
}

// handleAdminDevSeed serves POST /v1/admin/dev/seed — a DEV-ONLY helper that populates
// the store with demo users, subscriptions, and usage so the operator console isn't
// empty during local development. Returns 404 unless DevMode is on (so it never exists
// in production), and is otherwise admin-gated. Idempotent: skips if users already exist.
func handleAdminDevSeed(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.DevMode {
			writeErrorJSON(w, ErrorReasonNotFound, "not found")
			return
		}
		// Own guard (not adminGuard, which requires an {id} path value this route lacks).
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		admin, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
			return
		}
		if !cfg.IsAdmin(admin.ID) {
			auditFromRequest(r, store, admin, "admin.dev.seed.denied", "admin", "dev")
			writeErrorJSON(w, ErrorReasonPermissionDenied, "admin access required")
			return
		}
		existing, err := store.ListUsers(1, 0)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "seed precheck failed")
			return
		}
		if len(existing) > 0 {
			writeJSON(w, AdminActionResponse{OK: true, Action: "seed", Detail: "store already has users; skipped"})
			return
		}
		n, err := seedDemoData(store, time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "seed failed: "+err.Error())
			return
		}
		auditFromRequest(r, store, admin, "admin.dev.seed", "dev", "demo-users")
		writeJSON(w, AdminActionResponse{OK: true, Action: "seed", Detail: fmt.Sprintf("seeded %d demo users", n)})
	}
}

// seedDemoData inserts a spread of demo users with subscriptions and recent usage.
func seedDemoData(store *Store, now time.Time) (int, error) {
	type demo struct {
		id, provider, email, plan, status string
		reqs, toks                        int64
	}
	demos := []demo{
		{"demo-alice", "github", "alice@demo.dev", "monthly", "active", 42, 18400},
		{"demo-bob", "google", "bob@demo.dev", "annual", "active", 17, 9100},
		{"demo-carol", "github", "carol@demo.dev", "monthly", "trialing", 88, 31200},
		{"demo-dave", "google", "dave@demo.dev", "monthly", "past_due", 4, 1200},
		{"demo-erin", "github", "erin@demo.dev", "annual", "canceled", 0, 0},
		{"demo-frank", "google", "frank@demo.dev", "monthly", "active", 61, 24800},
		{"demo-grace", "github", "grace@demo.dev", "annual", "active", 9, 5300},
		{"demo-heidi", "google", "heidi@demo.dev", "monthly", "trialing", 130, 47600},
	}
	count := 0
	for i, d := range demos {
		created := now.AddDate(0, 0, -(i*9 + 3))
		if err := store.UpsertUser(User{ID: d.id, Provider: d.provider, Subject: d.id, Email: d.email, CreatedAt: created}); err != nil {
			return count, err
		}
		if err := store.PutSubscription(Subscription{
			UserID: d.id, ProviderCustomer: "cus_" + d.id, ProviderSubscription: "sub_" + d.id,
			Status: d.status, Plan: d.plan, CurrentPeriodEnd: now.AddDate(0, 0, 20), UpdatedAt: now,
		}); err != nil {
			return count, err
		}
		// Spread usage across the last 12 days so the per-user usage chart has shape.
		for day := 0; day < 12; day++ {
			req := d.reqs - int64(day*2)
			tok := d.toks - int64(day*900)
			if req < 0 {
				req = 0
			}
			if tok < 0 {
				tok = 0
			}
			if req == 0 && tok == 0 {
				continue
			}
			if _, err := store.AddUsage(d.id, now.AddDate(0, 0, -day), req, tok); err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

// handleAdminUserDetail serves GET /v1/admin/users/{id}.
func handleAdminUserDetail(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, target, ok := adminGuard(cfg, store, w, r, "admin.user.detail", "user")
		if !ok {
			return
		}
		u, found, err := store.GetUserByID(target)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "user lookup failed")
			return
		}
		if !found {
			writeErrorJSON(w, ErrorReasonNotFound, "user not found")
			return
		}
		resp := AdminUserDetailResponse{
			ID: u.ID, Provider: u.Provider, Email: u.Email,
			CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
		}
		if sub, ok, err := store.GetSubscription(target); err == nil && ok {
			resp.SubscriptionPlan = sub.Plan
			resp.SubscriptionStatus = sub.Status
			if !sub.CurrentPeriodEnd.IsZero() {
				resp.CurrentPeriodEnd = sub.CurrentPeriodEnd.UTC().Format(time.RFC3339)
			}
			if !sub.TrialEnd.IsZero() {
				resp.TrialEnd = sub.TrialEnd.UTC().Format(time.RFC3339)
			}
		}
		if ws, err := store.ListWorkspaces(target, false); err == nil {
			resp.WorkspaceCount = len(ws)
		}
		if bytes, err := store.UserBlobBytes(target); err == nil {
			resp.BlobBytes = bytes
		}
		if usage, ok, err := store.GetUsage(target, time.Now().UTC()); err == nil && ok {
			resp.UsageTodayRequests = usage.Requests
			resp.UsageTodayTokens = usage.Tokens
		}
		auditFromRequest(r, store, admin, "admin.user.detail.read", "user", target)
		writeJSON(w, resp)
	}
}

// handleAdminUserUsage serves GET /v1/admin/users/{id}/usage?days=N (default 30, max 365).
func handleAdminUserUsage(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, target, ok := adminGuard(cfg, store, w, r, "admin.user.usage", "user")
		if !ok {
			return
		}
		days := 30
		if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				days = n
			}
		}
		if days > 365 {
			days = 365
		}
		since := time.Now().UTC().AddDate(0, 0, -(days - 1))
		usage, err := store.ListUserUsage(target, since)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "usage lookup failed")
			return
		}
		auditFromRequest(r, store, admin, "admin.user.usage.read", "user", target)
		writeJSON(w, AdminUsageHistoryResponse{UserID: target, Days: days, Usage: usage})
	}
}

// handleAdminUserSetPlan serves POST /v1/admin/users/{id}/plan — overrides the plan
// and/or status on the user's existing subscription. Requires an existing subscription
// (Stripe creates it); this is a correction, not a creation.
func handleAdminUserSetPlan(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, target, ok := adminGuard(cfg, store, w, r, "admin.user.setPlan", "user")
		if !ok {
			return
		}
		var req AdminSetPlanRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid request body")
			return
		}
		req.Plan = strings.TrimSpace(req.Plan)
		req.Status = strings.TrimSpace(req.Status)
		if req.Plan == "" && req.Status == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "plan or status is required")
			return
		}
		if req.Status != "" && !validSubscriptionStatus(req.Status) {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "status must be one of: active, trialing, past_due, canceled, none")
			return
		}
		sub, found, err := store.GetSubscription(target)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "subscription lookup failed")
			return
		}
		action := "setPlan"
		if !found {
			// No subscription yet: an operator setting a plan/status creates one — the
			// comp/manual path. Synthesize a provider-neutral "manual" record with
			// unique customer/subscription ids so the UNIQUE constraints hold, and sane
			// defaults (perpetual active with no period end) when only one field is given.
			sub = Subscription{
				UserID:               target,
				Provider:             "manual",
				ProviderCustomer:     "manual:" + target,
				ProviderSubscription: "manual:" + target,
				Status:               "active",
				Plan:                 "comp",
			}
			action = "createPlan"
		}
		if req.Plan != "" {
			sub.Plan = req.Plan
		}
		if req.Status != "" {
			sub.Status = strings.ToLower(req.Status)
		}
		sub.UpdatedAt = time.Now().UTC()
		if err := store.PutSubscription(sub); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "subscription update failed")
			return
		}
		auditFromRequest(r, store, admin, "admin.user."+action, "user", target)
		writeJSON(w, AdminActionResponse{OK: true, Action: action, UserID: target, Detail: sub.Plan + "/" + sub.Status})
	}
}

// handleAdminUserRevokeSessions serves POST /v1/admin/users/{id}/revoke-sessions —
// invalidates all of a user's refresh sessions (forces re-login on every device).
func handleAdminUserRevokeSessions(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, target, ok := adminGuard(cfg, store, w, r, "admin.user.revokeSessions", "user")
		if !ok {
			return
		}
		if err := store.RevokeRefreshSessionsForUser(target, time.Now().UTC()); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "revoke failed")
			return
		}
		auditFromRequest(r, store, admin, "admin.user.revokeSessions", "user", target)
		writeJSON(w, AdminActionResponse{OK: true, Action: "revokeSessions", UserID: target})
	}
}

// handleAdminUserDelete serves DELETE /v1/admin/users/{id} — hard-deletes the account
// and all its server-side data. Destructive and audited; an admin cannot delete their
// own account through this route (guards against accidental self-lockout).
func handleAdminUserDelete(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, target, ok := adminGuard(cfg, store, w, r, "admin.user.delete", "user")
		if !ok {
			return
		}
		if target == admin.ID {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "an admin cannot delete their own account here")
			return
		}
		existed, err := store.DeleteAccount(target)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "delete failed")
			return
		}
		if !existed {
			writeErrorJSON(w, ErrorReasonNotFound, "user not found")
			return
		}
		auditFromRequest(r, store, admin, "admin.user.delete", "user", target)
		writeJSON(w, AdminActionResponse{OK: true, Action: "delete", UserID: target})
	}
}
