// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AdminOverviewResponse is the JSON body returned by GET /v1/admin/overview.
type AdminOverviewResponse struct {
	TotalUsers        int64  `json:"totalUsers"`
	SubsActive        int64  `json:"subsActive"`
	SubsTrialing      int64  `json:"subsTrialing"`
	SubsPastDue       int64  `json:"subsPastDue"`
	SubsCanceled      int64  `json:"subsCanceled"`
	EstimatedMRRCents int64  `json:"estimatedMrrCents"`
	TotalBlobBytes    int64  `json:"totalBlobBytes"`
	TodayRequests     int64  `json:"todayRequests"`
	TodayTokens       int64  `json:"todayTokens"`
	Day               string `json:"day"`
}

// AdminUsersResponse is the JSON body returned by GET /v1/admin/users.
type AdminUsersResponse struct {
	Users  []AdminUserRow `json:"users"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// handleAdminOverview serves GET /v1/admin/overview — admin-gated, audited.
func handleAdminOverview(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
			return
		}
		if !cfg.IsAdmin(user.ID) {
			auditFromRequest(r, store, user, "admin.overview.denied", "admin", "overview")
			writeErrorJSON(w, ErrorReasonPermissionDenied, "admin access required")
			return
		}
		today := time.Now().UTC()
		stats, err := store.AdminOverview(today)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "overview lookup failed")
			return
		}
		auditFromRequest(r, store, user, "admin.overview.read", "admin", "overview")
		writeJSON(w, AdminOverviewResponse{
			TotalUsers:        stats.TotalUsers,
			SubsActive:        stats.SubsActive,
			SubsTrialing:      stats.SubsTrialing,
			SubsPastDue:       stats.SubsPastDue,
			SubsCanceled:      stats.SubsCanceled,
			EstimatedMRRCents: stats.EstimatedMRRCents,
			TotalBlobBytes:    stats.TotalBlobBytes,
			TodayRequests:     stats.TodayRequests,
			TodayTokens:       stats.TodayTokens,
			Day:               today.Format("2006-01-02"),
		})
	}
}

// handleAdminUsers serves GET /v1/admin/users?limit=&offset= — admin-gated, audited.
// Defaults: limit=50, max=200. Returns no secrets, no AI ciphertext, no blob bytes.
func handleAdminUsers(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
			return
		}
		if !cfg.IsAdmin(user.ID) {
			auditFromRequest(r, store, user, "admin.users.denied", "admin", "users")
			writeErrorJSON(w, ErrorReasonPermissionDenied, "admin access required")
			return
		}
		limit := 50
		offset := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				offset = n
			}
		}
		// Cap and floor enforced inside ListUsers; mirror them here for the response.
		if limit > 200 {
			limit = 200
		}
		if limit <= 0 {
			limit = 50
		}
		users, err := store.ListUsers(limit, offset)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "users lookup failed")
			return
		}
		auditFromRequest(r, store, user, "admin.users.read", "admin", "users")
		writeJSON(w, AdminUsersResponse{
			Users:  users,
			Limit:  limit,
			Offset: offset,
		})
	}
}
