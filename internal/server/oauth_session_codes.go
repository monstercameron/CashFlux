// SPDX-License-Identifier: MIT

package server

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
)

const oauthSessionCodeTTL = 60 * time.Second

// migrateTo21 adds short-lived, hashed authorization codes. Schema and version
// advance together; a failed migration leaves neither one partially installed.
func (s *Store) migrateTo21() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("server store: migrate v21 begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS oauth_session_codes (
code_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
return_to TEXT NOT NULL, challenge TEXT NOT NULL, expires_at INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS oauth_session_codes_expiry ON oauth_session_codes(expires_at)`,
		`INSERT INTO schema_meta(id, version) VALUES(1, 21) ON CONFLICT(id) DO UPDATE SET version = excluded.version`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("server store: migrate v21 execute: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server store: migrate v21 commit: %w", err)
	}
	return nil
}

func (s *Store) issueOAuthSessionCode(userID, returnTo, challenge string, now time.Time) (string, error) {
	if userID == "" || returnTo == "" || !validPKCEChallenge(challenge) {
		return "", fmt.Errorf("server session: incomplete authorization code binding")
	}
	code, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("server session: generate authorization code: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("server session: begin authorization code: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM oauth_session_codes WHERE expires_at <= ?`, now.UnixNano()); err != nil {
		return "", fmt.Errorf("server session: prune authorization codes: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO oauth_session_codes(code_hash, user_id, return_to, challenge, expires_at) VALUES(?, ?, ?, ?, ?)`, sessionTokenHash(code), userID, returnTo, challenge, now.Add(oauthSessionCodeTTL).UnixNano()); err != nil {
		return "", fmt.Errorf("server session: store authorization code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("server session: commit authorization code: %w", err)
	}
	return code, nil
}

// redeemOAuthSessionCode consumes the code and persists its first refresh
// credential in one transaction. No bearer credential exists until redemption.
// Failed signing/storage rolls back consumption; concurrent redemptions have
// exactly one winner. The deadline is exclusive, including the 60-second boundary.
func (s *Store) redeemOAuthSessionCode(cfg Config, code, verifier, returnTo string, now time.Time) (backendrpc.TokenPairResponse, string, bool, error) {
	empty := backendrpc.TokenPairResponse{}
	if !validPKCEVerifier(verifier) {
		return empty, "", false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: begin code exchange: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var userID string
	err = tx.QueryRow(`DELETE FROM oauth_session_codes
WHERE code_hash = ? AND challenge = ? AND return_to = ? AND expires_at > ?
AND EXISTS (SELECT 1 FROM users WHERE users.id = oauth_session_codes.user_id AND suspended_at = '')
AND NOT EXISTS (SELECT 1 FROM deleted_accounts WHERE deleted_accounts.user_id = oauth_session_codes.user_id)
RETURNING user_id`, sessionTokenHash(code), pkceChallenge(verifier), returnTo, now.UnixNano()).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return empty, "", false, nil
	}
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: consume authorization code: %w", err)
	}
	family, err := randomURLToken(24)
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: generate code session family: %w", err)
	}
	jti, err := randomURLToken(24)
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: generate code session id: %w", err)
	}
	session, err := makeRefreshSuccessor(cfg, userID, family, jti, now, "")
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: sign code session: %w", err)
	}
	pair, err := tokenPairFromSuccessor(cfg, session, now)
	if err != nil {
		return empty, "", false, fmt.Errorf("server session: sign code token pair: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO refresh_tokens(jti, family_id, user_id, token_hash, device_label, expires_at, used_at, revoked_at) VALUES(?, ?, ?, ?, '', ?, '', '')`, session.JTI, session.FamilyID, userID, session.TokenHash, formatTime(session.ExpiresAt)); err != nil {
		return empty, "", false, fmt.Errorf("server session: store code session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return empty, "", false, fmt.Errorf("server session: commit code session: %w", err)
	}
	return pair, userID, true, nil
}
