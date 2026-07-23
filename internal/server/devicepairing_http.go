// SPDX-License-Identifier: MIT

package server

import "net/http"

// pairingCodeResponse is the response to POST /v1/devices/pair: a short-lived,
// single-use code the portal shows the user so they can link a new device via
// AuthService.RedeemPairingCode (TODOS.md C421).
type pairingCodeResponse struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expiresAt"`
}

// handleMintPairingCode mints a new pairing code for the authenticated portal
// user. It follows the same bearer-auth, no-CSRF shape as the other portal
// REST endpoints (handleAccountExport/handleBillingCheckout) — this call
// only ever mints a code for the caller's own account, never anyone else's.
func handleMintPairingCode(cfg Config, store *Store) http.HandlerFunc {
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
		code, expiresAt, err := store.MintPairingCode(user.ID, timeNowUTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "pairing code mint failed")
			return
		}
		auditFromRequest(r, store, user, "auth.pairing.mint", "user", user.ID)
		writeJSON(w, pairingCodeResponse{Code: code, ExpiresAt: formatTime(expiresAt)})
	}
}
