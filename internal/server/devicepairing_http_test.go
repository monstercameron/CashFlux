// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHandleMintPairingCodeIsRateLimited proves POST /v1/devices/pair is
// covered by authLimiter like the other unauthenticated-adjacent auth routes
// (/v1/auth/refresh, /v1/auth/logout, ...). Before this test's accompanying
// fix, this route was registered with a bare mux.HandleFunc — no rate limit
// at all — so an authenticated caller could mint pairing codes (each a
// fresh, 5-minute-lived, 6-digit account-takeover credential, see
// pairingcode.go) at an unbounded rate, growing both storage and the
// standing attack surface for free.
func TestHandleMintPairingCodeIsRateLimited(t *testing.T) {
	store := openTestStore(t)
	user := authUserFromToken("dev-token")
	seedSyncUser(t, store, user.ID, time.Now().UTC())
	h := NewMux(Config{
		AuthMode:               "token",
		Token:                  "dev-token",
		AppOrigin:              "http://127.0.0.1:8080",
		AuthRateLimitPerMinute: 1,
	}, store)

	mint := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/devices/pair", nil)
		req.Header.Set("Authorization", "Bearer dev-token")
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	if code := mint("198.51.100.9:1111"); code != http.StatusOK {
		t.Fatalf("first mint status = %d, want 200", code)
	}
	if code := mint("198.51.100.9:2222"); code != http.StatusTooManyRequests {
		t.Fatalf("second mint from the same IP status = %d, want 429 (rate limited)", code)
	}
}
