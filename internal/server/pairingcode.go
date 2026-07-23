// SPDX-License-Identifier: MIT

package server

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// PairingCodeTTL is how long a minted pairing code stays redeemable
// (TODOS.md C421). Short-lived by design: it is displayed once in the portal
// and typed/scanned on the new device within the same sitting.
const PairingCodeTTL = 5 * time.Minute

// pairingCodeDigits is the length of a minted pairing code. Six digits keeps
// it easy to type by hand while giving a 1-in-a-million guess space over the
// 5-minute TTL.
const pairingCodeDigits = 6

// pairingCodeMintAttempts bounds retries on the (astronomically unlikely)
// event a freshly minted code collides with one already outstanding.
const pairingCodeMintAttempts = 5

// ErrPairingCodeExhausted is returned by MintPairingCode if it could not find
// an unused code after pairingCodeMintAttempts tries.
var ErrPairingCodeExhausted = errors.New("server store: could not mint a unique pairing code")

// MintPairingCode creates a new short-lived, single-use pairing code tied to
// userID, for RedeemPairingCode (TODOS.md C421) to later resolve back to the
// same, already-existing account. It never mints a code for the purpose of
// creating a new account.
func (s *Store) MintPairingCode(userID string, now time.Time) (string, time.Time, error) {
	if s == nil || s.db == nil {
		return "", time.Time{}, fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", time.Time{}, fmt.Errorf("server store: user id is required")
	}
	defer s.observeDB("MintPairingCode", time.Now())
	expiresAt := now.Add(PairingCodeTTL)
	for attempt := 0; attempt < pairingCodeMintAttempts; attempt++ {
		code, err := randomPairingCode()
		if err != nil {
			return "", time.Time{}, fmt.Errorf("server store: generate pairing code: %w", err)
		}
		_, err = s.db.Exec(`INSERT INTO pairing_codes(code, user_id, created_at, expires_at, consumed_at) VALUES(?, ?, ?, ?, '')`,
			code, userID, formatTime(now), formatTime(expiresAt))
		if err == nil {
			return code, expiresAt, nil
		}
		if !isUniqueConstraintErr(err) {
			return "", time.Time{}, fmt.Errorf("server store: mint pairing code: %w", err)
		}
		// Collision on the PRIMARY KEY(code) — vanishingly rare; retry with a fresh code.
	}
	return "", time.Time{}, ErrPairingCodeExhausted
}

// ConsumePairingCode looks up a pairing code, verifies it is unexpired and not
// already consumed, atomically marks it consumed, and returns the user id it
// was minted for. It is single-use: a second redemption of the same code
// (replay, race between two devices) always fails, matching TODOS.md C421's
// requirement that this path only ever resolve an existing account.
func (s *Store) ConsumePairingCode(code string, now time.Time) (userID string, ok bool, err error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("server store: not configured")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false, nil
	}
	defer s.observeDB("ConsumePairingCode", time.Now())

	tx, err := s.db.Begin()
	if err != nil {
		return "", false, fmt.Errorf("server store: consume pairing code: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		rowUserID  string
		expiresRaw string
		consumed   string
	)
	err = tx.QueryRow(`SELECT user_id, expires_at, consumed_at FROM pairing_codes WHERE code = ?`, code).
		Scan(&rowUserID, &expiresRaw, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("server store: lookup pairing code: %w", err)
	}
	if strings.TrimSpace(consumed) != "" {
		return "", false, nil
	}
	expiresAt, err := parseTime(expiresRaw)
	if err != nil {
		return "", false, fmt.Errorf("server store: parse pairing code expiry: %w", err)
	}
	if !now.Before(expiresAt) {
		return "", false, nil
	}
	res, err := tx.Exec(`UPDATE pairing_codes SET consumed_at = ? WHERE code = ? AND consumed_at = ''`, formatTime(now), code)
	if err != nil {
		return "", false, fmt.Errorf("server store: consume pairing code: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return "", false, fmt.Errorf("server store: consume pairing code: %w", err)
	}
	if affected == 0 {
		// Another concurrent redemption won the race between the SELECT and this UPDATE.
		return "", false, nil
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("server store: consume pairing code: %w", err)
	}
	return rowUserID, true, nil
}

// PeekPairingCodeUserID looks up the user id a pairing code was minted for,
// without consuming it or checking expiry/consumed state — the read half
// RedeemPairingCode's idempotency handling (TODOS.md C443) needs: a retried
// RedeemPairingCode call must resolve the SAME already-consumed code back to
// its user id so the cached token pair can be replayed, which
// ConsumePairingCode alone cannot do (it deliberately fails on a second
// consume). ok is false only if code was never minted at all.
func (s *Store) PeekPairingCodeUserID(code string) (userID string, ok bool, err error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("server store: not configured")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false, nil
	}
	defer s.observeDB("PeekPairingCodeUserID", time.Now())
	err = s.db.QueryRow(`SELECT user_id FROM pairing_codes WHERE code = ?`, code).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("server store: peek pairing code: %w", err)
	}
	return userID, true, nil
}

// randomPairingCode returns a zero-padded, cryptographically random numeric
// code of pairingCodeDigits digits (e.g. "042817").
func randomPairingCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < pairingCodeDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", pairingCodeDigits, n.Int64()), nil
}

// isUniqueConstraintErr reports whether err looks like a SQLite unique/primary
// key constraint violation. The pure-Go ncruces/go-sqlite3 driver surfaces
// this as a plain error whose text contains "UNIQUE constraint failed" (there
// is no typed sentinel to compare against here).
func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
