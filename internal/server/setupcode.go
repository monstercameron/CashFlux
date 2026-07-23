// SPDX-License-Identifier: MIT

package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// setupCodeHash returns the sha256 hex digest a setup code is tracked under
// in the setup_codes table — the plaintext code (an operator-distributed env
// var value) is never persisted.
func setupCodeHash(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

// SetupCodeAvailable reports whether code has not yet been consumed, without
// consuming it. AuthService.RequestPhoneVerification uses this as a fail-fast
// check (TODOS.md portfolio-embedding gate) so an already-spent code doesn't
// waste an SMS before the caller ever reaches VerifyPhoneCode. code must
// already be known to equal Config.SetupCode — callers compare the caller-
// supplied value against it (constant-time) themselves before calling this;
// this method only tracks single-use consumption, it does not authenticate
// the value.
func (s *Store) SetupCodeAvailable(code string) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	if strings.TrimSpace(code) == "" {
		return false, nil
	}
	defer s.observeDB("SetupCodeAvailable", time.Now())
	var consumed string
	err := s.db.QueryRow(`SELECT consumed_at FROM setup_codes WHERE code_hash = ?`, setupCodeHash(code)).Scan(&consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: lookup setup code: %w", err)
	}
	return consumed == "", nil
}

// ConsumeSetupCode atomically marks code as spent and reports whether this
// call was the one that consumed it (false if it was already consumed).
// AuthService.VerifyPhoneCode calls this only on successful phone
// verification, so a fumbled SMS attempt never burns the invite. As with
// SetupCodeAvailable, code must already be known to equal Config.SetupCode —
// this method does not authenticate the value, only tracks its single use.
func (s *Store) ConsumeSetupCode(code string, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	if strings.TrimSpace(code) == "" {
		return false, nil
	}
	defer s.observeDB("ConsumeSetupCode", time.Now())
	hash := setupCodeHash(code)
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO setup_codes(code_hash, consumed_at) VALUES(?, '')`, hash); err != nil {
		return false, fmt.Errorf("server store: consume setup code: %w", err)
	}
	res, err := s.db.Exec(`UPDATE setup_codes SET consumed_at = ? WHERE code_hash = ? AND consumed_at = ''`, formatTime(now), hash)
	if err != nil {
		return false, fmt.Errorf("server store: consume setup code: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: consume setup code: %w", err)
	}
	return affected > 0, nil
}

// PhoneVerifiedBefore reports whether userID has ever completed
// VerifyPhoneCode. AuthService uses this — not user-row existence, which
// ensurePhoneUser creates eagerly before any code is checked — to tell a
// genuinely new account from a returning phone number signing in on another
// device, so the setup-code gate applies only to the former.
func (s *Store) PhoneVerifiedBefore(userID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, nil
	}
	defer s.observeDB("PhoneVerifiedBefore", time.Now())
	var verifiedAt string
	err := s.db.QueryRow(`SELECT phone_verified_at FROM users WHERE id = ?`, userID).Scan(&verifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: phone verified before: %w", err)
	}
	return strings.TrimSpace(verifiedAt) != "", nil
}

// MarkPhoneVerified stamps userID's phone_verified_at on a successful
// VerifyPhoneCode. Idempotent: a later call from a session replay just
// rewrites the same kind of timestamp.
func (s *Store) MarkPhoneVerified(userID string, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("server store: user id is required")
	}
	defer s.observeDB("MarkPhoneVerified", time.Now())
	if _, err := s.db.Exec(`UPDATE users SET phone_verified_at = ? WHERE id = ?`, formatTime(now), userID); err != nil {
		return fmt.Errorf("server store: mark phone verified: %w", err)
	}
	return nil
}
