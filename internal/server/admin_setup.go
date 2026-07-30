// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdminSetupStatusResponse tells the console whether this installation still
// needs its first persisted owner account.
type AdminSetupStatusResponse struct {
	Required bool `json:"required"`
}

// AdminSetupRequest is accepted exactly once. SetupKey is the server's existing
// static break-glass credential, which prevents the first random visitor from
// claiming an uninitialized internet-facing installation.
type AdminSetupRequest struct {
	SetupKey string `json:"setupKey"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// AdminSetupResponse returns the new owner and its one-time recovery code. Only
// the bcrypt recovery hash is persisted.
type AdminSetupResponse struct {
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	RecoveryCode string `json:"recoveryCode"`
}

func handleAdminSetupStatus(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		hasOwner, err := store.HasOwner()
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "owner setup status failed")
			return
		}
		writeJSON(w, AdminSetupStatusResponse{Required: !hasOwner})
	}
}

func handleAdminSetupCreate(cfg Config, store *Store, setupMu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if store == nil || setupMu == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}

		var req AdminSetupRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid request body")
			return
		}
		req.SetupKey = strings.TrimSpace(req.SetupKey)
		req.Username = strings.TrimSpace(req.Username)
		if !cfg.matchesStaticToken(req.SetupKey) {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "setup key is invalid")
			return
		}
		if req.Username == "" || req.Password == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "username and password are required")
			return
		}
		if len(req.Username) > maxUsernameLength {
			writeErrorJSON(w, ErrorReasonInvalidArgument, fmt.Sprintf("username must be at most %d characters", maxUsernameLength))
			return
		}
		if len(req.Password) < minLocalPasswordLength {
			writeErrorJSON(w, ErrorReasonInvalidArgument, fmt.Sprintf("password must be at least %d characters", minLocalPasswordLength))
			return
		}

		// Serialize the check-and-create pair so two simultaneous first-run
		// submissions cannot mint two owners in this server process.
		setupMu.Lock()
		defer setupMu.Unlock()
		hasOwner, err := store.HasOwner()
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "owner setup status failed")
			return
		}
		if hasOwner {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "owner setup is already complete")
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "password hashing failed")
			return
		}
		recoveryCode, err := generateRecoveryCode()
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "recovery code generation failed")
			return
		}
		recoveryHash, err := bcrypt.GenerateFromPassword([]byte(recoveryCode), bcrypt.DefaultCost)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "recovery code hashing failed")
			return
		}
		user, err := store.CreateLocalUserWithRole(
			req.Username,
			string(passwordHash),
			string(recoveryHash),
			RoleOwner,
			time.Now().UTC(),
		)
		switch {
		case errors.Is(err, ErrUsernameTaken):
			writeErrorJSON(w, ErrorReasonInvalidArgument, "username is already registered")
			return
		case err != nil:
			writeErrorJSON(w, ErrorReasonInternal, "owner account creation failed")
			return
		}
		auditFromRequest(r, store, AuthUser{ID: user.ID}, "admin.setup.complete", "user", user.ID)
		writeJSON(w, AdminSetupResponse{
			UserID:       user.ID,
			Username:     user.Username,
			RecoveryCode: recoveryCode,
		})
	}
}
