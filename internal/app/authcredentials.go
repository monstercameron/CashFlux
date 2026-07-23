// SPDX-License-Identifier: MIT

// Package app — pure validation helpers for the username/password fallback and
// pairing-code device-linking forms (TODOS.md C421/C422 client UI). These are
// platform-independent (no syscall/js) so they unit-test on native Go; the
// wasm view code (authcards.go) calls them before ever dialing the backend,
// and maps the returned sentinel errors to translated copy via uistate.T.
package app

import (
	"errors"
	"strings"
)

// authMinPasswordLength mirrors internal/server/authservice.go's
// minLocalPasswordLength: the client validates up front so a too-short
// password never round-trips to the server just to be rejected. internal/app
// cannot import internal/server directly (the server package pulls in
// database/network dependencies that don't build for js/wasm), so the value
// is duplicated here; keep the two in lockstep by hand.
const authMinPasswordLength = 8

// pairingCodeLength mirrors internal/server/pairingcode.go's
// pairingCodeDigits: a minted pairing code is always exactly this many
// digits. Duplicated for the same cross-package reason as
// authMinPasswordLength.
const pairingCodeLength = 6

// Sentinel validation errors for the password and pairing-code forms. The
// view layer switches on these (never on err.Error()) to pick a translated,
// user-facing message — see authcards.go.
var (
	ErrUsernameRequired   = errors.New("app: username is required")
	ErrPasswordRequired   = errors.New("app: password is required")
	ErrPasswordTooShort   = errors.New("app: password is too short")
	ErrPairingCodeMissing = errors.New("app: pairing code is required")
	ErrPairingCodeInvalid = errors.New("app: pairing code must be digits of the expected length")
)

// normalizeUsername trims surrounding whitespace, the only normalization a
// username fallback account needs — usernames are otherwise taken verbatim
// (case-sensitive, no charset restriction beyond what the server enforces).
func normalizeUsername(raw string) string {
	return strings.TrimSpace(raw)
}

// validateRegisterCredentials checks a username/password pair client-side
// before AuthService.Register is called: both fields present, and the
// password at least authMinPasswordLength long (the server's own floor).
func validateRegisterCredentials(username, password string) error {
	if normalizeUsername(username) == "" {
		return ErrUsernameRequired
	}
	if password == "" {
		return ErrPasswordRequired
	}
	if len(password) < authMinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// validateLoginCredentials checks a username/password pair client-side before
// AuthService.Login is called: both fields simply need to be present — the
// server alone knows whether they're correct.
func validateLoginCredentials(username, password string) error {
	if normalizeUsername(username) == "" {
		return ErrUsernameRequired
	}
	if password == "" {
		return ErrPasswordRequired
	}
	return nil
}

// normalizePairingCode strips whitespace the user may have typed around or
// inside the code (a pairing code is often read aloud or copied with a
// grouping space, e.g. "123 456") and verifies what remains is exactly
// pairingCodeLength digits. It never guesses beyond that — no dashes or other
// separators are stripped, since the portal never mints codes containing
// them.
func normalizePairingCode(raw string) (string, error) {
	code := strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
	if code == "" {
		return "", ErrPairingCodeMissing
	}
	if len(code) != pairingCodeLength {
		return "", ErrPairingCodeInvalid
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", ErrPairingCodeInvalid
		}
	}
	return code, nil
}
