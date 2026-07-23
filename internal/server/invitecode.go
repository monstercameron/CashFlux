// SPDX-License-Identifier: MIT

package server

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// InviteCodeTTL is how long a minted enrollment invite code stays redeemable.
// Short-lived by design (mirroring PairingCodeTTL's reasoning), but longer
// than a pairing code's 5 minutes since an invite is typically relayed to
// someone out-of-band (a text message) before they act on it, not typed on
// the same device in the same sitting.
const InviteCodeTTL = 15 * time.Minute

// inviteCodeDigits is the length of a minted invite code.
const inviteCodeDigits = 6

// inviteCodeMintAttempts bounds retries on the (astronomically unlikely)
// event a freshly minted code collides with one already outstanding.
const inviteCodeMintAttempts = 5

// ErrInviteCodeExhausted is returned by MintInviteCode if it could not find
// an unused code after inviteCodeMintAttempts tries.
var ErrInviteCodeExhausted = errors.New("server store: could not mint a unique invite code")

// InviteCodeRow is one minted invite code, for admin listing
// (pkg/embed.Admin.ListInviteCodes). ConsumedAt is the zero time when the
// code is still outstanding.
type InviteCodeRow struct {
	Code       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt time.Time
}

// MintInviteCode creates a new short-lived, single-use enrollment invite
// code not tied to any existing account — the admin-driven companion to
// Config.SetupCode (TODOS.md portfolio-embedding gate): an operator mints
// one from the admin console and hands it to a specific person, rather than
// sharing one static secret with everyone they ever invite.
func (s *Store) MintInviteCode(now time.Time) (string, time.Time, error) {
	if s == nil || s.db == nil {
		return "", time.Time{}, fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("MintInviteCode", time.Now())
	expiresAt := now.Add(InviteCodeTTL)
	for attempt := 0; attempt < inviteCodeMintAttempts; attempt++ {
		code, err := randomInviteCode()
		if err != nil {
			return "", time.Time{}, fmt.Errorf("server store: generate invite code: %w", err)
		}
		_, err = s.db.Exec(`INSERT INTO invite_codes(code, created_at, expires_at, consumed_at) VALUES(?, ?, ?, '')`,
			code, formatTime(now), formatTime(expiresAt))
		if err == nil {
			return code, expiresAt, nil
		}
		if !isUniqueConstraintErr(err) {
			return "", time.Time{}, fmt.Errorf("server store: mint invite code: %w", err)
		}
		// Collision on the PRIMARY KEY(code) — vanishingly rare; retry with a fresh code.
	}
	return "", time.Time{}, ErrInviteCodeExhausted
}

// InviteCodeAvailable reports whether code was minted, is unexpired, and has
// not been consumed, without consuming it. Unlike SetupCodeAvailable (which
// treats an unrecognized value as available, safe only because its caller
// already proved the value equals Config.SetupCode), a code never minted
// here must always read as unavailable — this table, not an env var, is the
// only source of truth for which invite codes are real.
func (s *Store) InviteCodeAvailable(code string, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	if code == "" {
		return false, nil
	}
	defer s.observeDB("InviteCodeAvailable", time.Now())
	var (
		expiresRaw string
		consumed   string
	)
	err := s.db.QueryRow(`SELECT expires_at, consumed_at FROM invite_codes WHERE code = ?`, code).Scan(&expiresRaw, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: lookup invite code: %w", err)
	}
	if consumed != "" {
		return false, nil
	}
	expiresAt, err := parseTime(expiresRaw)
	if err != nil {
		return false, fmt.Errorf("server store: parse invite code expiry: %w", err)
	}
	return now.Before(expiresAt), nil
}

// ConsumeInviteCode looks up an invite code, verifies it is unexpired and not
// already consumed, and atomically marks it consumed. It is single-use: a
// second redemption of the same code (replay, race between two enrollments)
// always fails. Mirrors ConsumePairingCode's SELECT-then-atomic-UPDATE
// pattern, minus the user id (an invite code has no associated account yet).
func (s *Store) ConsumeInviteCode(code string, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	if code == "" {
		return false, nil
	}
	defer s.observeDB("ConsumeInviteCode", time.Now())

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("server store: consume invite code: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		expiresRaw string
		consumed   string
	)
	err = tx.QueryRow(`SELECT expires_at, consumed_at FROM invite_codes WHERE code = ?`, code).Scan(&expiresRaw, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: lookup invite code: %w", err)
	}
	if consumed != "" {
		return false, nil
	}
	expiresAt, err := parseTime(expiresRaw)
	if err != nil {
		return false, fmt.Errorf("server store: parse invite code expiry: %w", err)
	}
	if !now.Before(expiresAt) {
		return false, nil
	}
	res, err := tx.Exec(`UPDATE invite_codes SET consumed_at = ? WHERE code = ? AND consumed_at = ''`, formatTime(now), code)
	if err != nil {
		return false, fmt.Errorf("server store: consume invite code: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: consume invite code: %w", err)
	}
	if affected == 0 {
		// Another concurrent redemption won the race between the SELECT and this UPDATE.
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("server store: consume invite code: %w", err)
	}
	return true, nil
}

// ListInviteCodes returns the most recently minted invite codes, newest
// first, capped at limit — for admin visibility (pkg/embed.Admin.ListInviteCodes).
// There is no cleanup/GC job for expired-and-unconsumed rows; the limit is
// what keeps a long-running deployment's list from growing unbounded, given
// the expected volume (a handful of invites, ever) makes real pagination
// unnecessary.
func (s *Store) ListInviteCodes(limit int) ([]InviteCodeRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("server store: not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	defer s.observeDB("ListInviteCodes", time.Now())
	rows, err := s.db.Query(`SELECT code, created_at, expires_at, consumed_at FROM invite_codes ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("server store: list invite codes: %w", err)
	}
	defer rows.Close()
	var out []InviteCodeRow
	for rows.Next() {
		var (
			code                            string
			createdRaw, expiresRaw, consRaw string
		)
		if err := rows.Scan(&code, &createdRaw, &expiresRaw, &consRaw); err != nil {
			return nil, fmt.Errorf("server store: scan invite code: %w", err)
		}
		createdAt, err := parseTime(createdRaw)
		if err != nil {
			return nil, fmt.Errorf("server store: parse invite code created_at: %w", err)
		}
		expiresAt, err := parseTime(expiresRaw)
		if err != nil {
			return nil, fmt.Errorf("server store: parse invite code expires_at: %w", err)
		}
		var consumedAt time.Time
		if consRaw != "" {
			consumedAt, err = parseTime(consRaw)
			if err != nil {
				return nil, fmt.Errorf("server store: parse invite code consumed_at: %w", err)
			}
		}
		out = append(out, InviteCodeRow{Code: code, CreatedAt: createdAt, ExpiresAt: expiresAt, ConsumedAt: consumedAt})
	}
	return out, rows.Err()
}

// randomInviteCode returns a zero-padded, cryptographically random numeric
// code of inviteCodeDigits digits — identical generation logic to
// randomPairingCode, duplicated rather than shared since each is a tiny,
// self-contained helper scoped to its own file's constants.
func randomInviteCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < inviteCodeDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", inviteCodeDigits, n.Int64()), nil
}
