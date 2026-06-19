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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "account not found", http.StatusNotFound)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}
		if _, err := store.SweepUnreferencedBlobs(blobRoot(cfg)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func authorizedAccountRequest(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	if !writeCORS(w, r, cfg) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return AuthUser{}, false
	}
	if store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return AuthUser{}, false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return AuthUser{}, false
	}
	SetLogScope(r.Context(), LogScope{UserID: user.ID})
	return user, true
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
