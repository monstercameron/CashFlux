// SPDX-License-Identifier: MIT

package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"
)

// localUserIDPrefix marks a User.ID as a username/password account (as
// opposed to "google:<subject>"/"github:<subject>" OAuth ids) so the id
// itself is sufficient to tell the two enrollment doors apart at a glance.
const localUserIDPrefix = "local:"

// localUserID derives the deterministic User.ID for a username/password
// account, matching the existing "provider:subject" id convention (see User
// in repository.go) so local accounts share the same users table and id
// shape as OAuth accounts rather than needing a parallel identity scheme.
func localUserID(username string) string {
	return localUserIDPrefix + username
}

// ErrUsernameTaken is returned by CreateLocalUser when username is already registered.
var ErrUsernameTaken = errors.New("server store: username is already registered")

// CreateLocalUser creates a new username/password account (TODOS.md C422).
// It fails with ErrUsernameTaken if the username is already registered;
// callers are expected to have already validated username/passwordHash are
// non-empty and passwordHash is a bcrypt hash, not a raw password.
func (s *Store) CreateLocalUser(username, passwordHash, recoveryCodeHash string, now time.Time) (User, error) {
	if s == nil || s.db == nil {
		return User{}, fmt.Errorf("server store: not configured")
	}
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(passwordHash) == "" {
		return User{}, fmt.Errorf("server store: username and password hash are required")
	}
	defer s.observeDB("CreateLocalUser", time.Now())
	id := localUserID(username)
	now = now.UTC()
	_, err := s.db.Exec(`
INSERT INTO users(id, provider, subject, email, created_at, password_hash, recovery_code_hash, username)
VALUES(?, 'local', ?, '', ?, ?, ?, ?)`,
		id, username, formatTime(now), passwordHash, recoveryCodeHash, username)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("server store: create local user: %w", err)
	}
	return User{ID: id, Provider: "local", Subject: username, CreatedAt: now}, nil
}

// GetLocalUserByUsername looks up an account by its login username and its
// bcrypt password hash for Login (TODOS.md C422). ok is false if no account
// has that username set. Deliberately keyed off the username COLUMN rather
// than deriving an id from username (as CreateLocalUser's `local:<username>`
// id does): SetPassword (TODOS.md C454) attaches a username/password to
// whatever account a session already belongs to — which may have originated
// from token-mode, OAuth, or device pairing, none of which use the `local:`
// id scheme — so the lookup must work for any provider, not just 'local'.
func (s *Store) GetLocalUserByUsername(username string) (user User, passwordHash string, ok bool, err error) {
	if s == nil || s.db == nil {
		return User{}, "", false, fmt.Errorf("server store: not configured")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, "", false, nil
	}
	defer s.observeDB("GetLocalUserByUsername", time.Now())
	var created string
	err = s.db.QueryRow(`
SELECT id, provider, subject, email, created_at, password_hash
FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Provider, &user.Subject, &user.Email, &created, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(passwordHash) == "" {
		return User{}, "", false, nil
	}
	if err != nil {
		return User{}, "", false, fmt.Errorf("server store: get local user: %w", err)
	}
	user.CreatedAt, err = parseTime(created)
	if err != nil {
		return User{}, "", false, fmt.Errorf("server store: parse local user time: %w", err)
	}
	return user, passwordHash, true, nil
}

// SetLocalCredentials attaches a login username and bcrypt password hash to
// an EXISTING user row (looked up by userID, never creating a new one) — the
// store half of SetPassword's pairing-bootstrap contract (TODOS.md C454):
// the account the caller is already signed in as gains a username/password,
// it does not get a second, disconnected account the way calling
// CreateLocalUser would. Fails with ErrUsernameTaken if another account
// already owns that username, and with ErrUserNotFound if userID doesn't
// exist (callers are expected to have already lazily materialized it — see
// SyncService.ensureUser's doc comment for why that lazy-creation exists).
func (s *Store) SetLocalCredentials(userID, username, passwordHash string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	username = strings.TrimSpace(username)
	if userID == "" || username == "" || strings.TrimSpace(passwordHash) == "" {
		return fmt.Errorf("server store: user id, username, and password hash are required")
	}
	defer s.observeDB("SetLocalCredentials", time.Now())
	res, err := s.db.Exec(`UPDATE users SET username = ?, password_hash = ? WHERE id = ?`,
		username, passwordHash, userID)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return ErrUsernameTaken
		}
		return fmt.Errorf("server store: set local credentials: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("server store: set local credentials: %w", err)
	}
	if affected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ErrUserNotFound is returned by SetLocalCredentials when userID does not
// name an existing users row.
var ErrUserNotFound = errors.New("server store: user not found")

// generateRecoveryCode returns a random, human-typeable one-time account
// recovery code (TODOS.md C422) shown to the caller exactly once at
// Register time — password accounts have no email/SMS-backed recovery path,
// so this is what stands in for "forgot password" until a ResetPassword
// door exists. Base32 (Crockford-free but unambiguous enough for a save-me
// note) keeps it free of visually confusable characters like 0/O and 1/I/l
// found in mixed-case alphanumerics.
func generateRecoveryCode() (string, error) {
	buf := make([]byte, 10) // 16 base32 chars, well above brute-force range for a hashed, unindexed secret
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("server store: generate recovery code: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}
