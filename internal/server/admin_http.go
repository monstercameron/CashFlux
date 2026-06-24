// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"time"
)

// AdminUsageResponse is a read-only support view of the authenticated user's daily usage.
type AdminUsageResponse struct {
	UserID   string `json:"userId"`
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

func handleAdminUsage(cfg Config, store *Store) http.HandlerFunc {
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
		day := time.Now().UTC()
		if raw := r.URL.Query().Get("day"); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid day")
				return
			}
			day = parsed
		}
		usage, ok, err := store.GetUsage(user.ID, day)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "usage lookup failed")
			return
		}
		if !ok {
			usage = Usage{UserID: user.ID, Day: usageDay(day)}
		}
		writeJSON(w, AdminUsageResponse{
			UserID:   usage.UserID,
			Day:      usage.Day,
			Requests: usage.Requests,
			Tokens:   usage.Tokens,
		})
	}
}
