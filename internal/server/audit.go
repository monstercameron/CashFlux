// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func auditFromContext(ctx context.Context, store *Store, action, targetType, targetID string) {
	if store == nil {
		return
	}
	user, ok := AuthUserFromContext(ctx)
	if !ok {
		return
	}
	requestID, _ := RequestIDFromContext(ctx)
	_, _ = store.AppendAuditEvent(AuditEvent{
		Timestamp:  time.Now().UTC(),
		ActorID:    user.ID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID,
	})
}

func auditFromRequest(r *http.Request, store *Store, user AuthUser, action, targetType, targetID string) {
	if store == nil || strings.TrimSpace(user.ID) == "" {
		return
	}
	requestID, _ := RequestIDFromContext(r.Context())
	_, _ = store.AppendAuditEvent(AuditEvent{
		Timestamp:  time.Now().UTC(),
		ActorID:    user.ID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		IP:         clientIP(r),
		RequestID:  requestID,
	})
}

func handleAuditEvents(cfg Config, store *Store) http.HandlerFunc {
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
		afterID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("afterId")), 10, 64)
		limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
		// Tenant isolation: only an operator-designated admin sees the GLOBAL audit
		// log (every user's actor_id, IP, and target ids). A regular Cloud user gets
		// ONLY their own actor-scoped events — the endpoint used to return the whole
		// log to any authenticated bearer, a cross-tenant leak in multi-tenant Cloud.
		var events []AuditEvent
		var err error
		if httpOperatorAuthorized(user, cfg) {
			events, err = store.ListAuditEvents(afterID, limit)
		} else {
			events, err = store.ListAuditEventsForActor(user.ID, afterID, limit)
		}
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "audit lookup failed")
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		for _, event := range events {
			if err := enc.Encode(event); err != nil {
				return
			}
		}
	}
}
