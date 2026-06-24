// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"time"
)

func handleAccountExport(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedAccountRequest(w, r, cfg, store)
		if !ok {
			return
		}
		export, found, err := store.ExportAccount(user.ID, timeNowUTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "account export failed")
			return
		}
		if !found {
			writeErrorJSON(w, ErrorReasonNotFound, "account not found")
			return
		}
		auditFromRequest(r, store, user, "account.export", "user", user.ID)
		writeJSON(w, export)
	}
}

func handleAccountDelete(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedAccountRequest(w, r, cfg, store)
		if !ok {
			return
		}
		auditFromRequest(r, store, user, "account.delete", "user", user.ID)
		deleted, err := store.DeleteAccount(user.ID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "account delete failed")
			return
		}
		if !deleted {
			writeErrorJSON(w, ErrorReasonNotFound, "account not found")
			return
		}
		if _, err := store.SweepUnreferencedBlobs(blobRoot(cfg)); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "blob cleanup failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func authorizedAccountRequest(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	if !writeCORS(w, r, cfg) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
		return AuthUser{}, false
	}
	if store == nil {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
		return AuthUser{}, false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
		return AuthUser{}, false
	}
	SetLogScope(r.Context(), LogScope{UserID: user.ID})
	return user, true
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
