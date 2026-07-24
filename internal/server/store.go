// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver" // registers the pure-Go sqlite3 driver
)

const CurrentServerSchemaVersion = 12
const sqliteBusyTimeoutMillis = 5000

// Store owns the backend SQLite database.
type Store struct {
	db      *sql.DB
	metrics *Metrics
}

// OpenStore opens or creates the server database at path and applies migrations.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("server store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("server store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = %d;", sqliteBusyTimeoutMillis)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("server store: pragmas: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DryRunStoreMigrations applies migrations to a temporary copy of path and
// returns the migrated schema version without mutating the live database.
func DryRunStoreMigrations(path string) (int, error) {
	tempDir, err := os.MkdirTemp("", "cashflux-migrate-check-*")
	if err != nil {
		return 0, fmt.Errorf("server store: migration dry-run tempdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	tempPath := filepath.Join(tempDir, filepath.Base(path))
	if _, err := os.Stat(path); err == nil {
		if err := copyStoreFile(path, tempPath); err != nil {
			return 0, err
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(path + suffix); err == nil {
				if err := copyStoreFile(path+suffix, tempPath+suffix); err != nil {
					return 0, err
				}
			} else if err != nil && !os.IsNotExist(err) {
				return 0, fmt.Errorf("server store: migration dry-run stat sidecar: %w", err)
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("server store: migration dry-run stat: %w", err)
	}
	store, err := OpenStore(tempPath)
	if err != nil {
		return 0, fmt.Errorf("server store: migration dry-run: %w", err)
	}
	defer func() { _ = store.Close() }()
	version, err := store.SchemaVersion()
	if err != nil {
		return 0, fmt.Errorf("server store: migration dry-run version: %w", err)
	}
	return version, nil
}

func copyStoreFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("server store: migration dry-run open %s: %w", filepath.Base(src), err)
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("server store: migration dry-run mkdir: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("server store: migration dry-run create %s: %w", filepath.Base(dst), err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("server store: migration dry-run copy %s: %w", filepath.Base(src), copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("server store: migration dry-run close %s: %w", filepath.Base(dst), closeErr)
	}
	return nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SetMetrics(metrics *Metrics) {
	if s != nil {
		s.metrics = metrics
	}
}

func (s *Store) observeDB(operation string, start time.Time) {
	if s != nil {
		s.metrics.ObserveDB(operation, time.Since(start))
	}
}

// CheckpointWAL flushes the SQLite write-ahead log back into the main database file.
func (s *Store) CheckpointWAL(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("CheckpointWAL", time.Now())
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("server store: checkpoint wal: %w", err)
	}
	return nil
}

// Ready verifies the backing store can serve requests.
func (s *Store) Ready() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("Ready", time.Now())
	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("server store: ping: %w", err)
	}
	if _, err := s.SchemaVersion(); err != nil {
		return err
	}
	return nil
}

// SchemaVersion returns the current migrated server schema version.
func (s *Store) SchemaVersion() (int, error) {
	defer s.observeDB("SchemaVersion", time.Now())
	var v int
	err := s.db.QueryRow("SELECT version FROM schema_meta WHERE id = 1").Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("server store: schema version: %w", err)
	}
	return v, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);`); err != nil {
		return fmt.Errorf("server store: schema meta: %w", err)
	}
	var version int
	err := s.db.QueryRow("SELECT version FROM schema_meta WHERE id = 1").Scan(&version)
	if err == sql.ErrNoRows {
		version = 0
	} else if err != nil {
		return fmt.Errorf("server store: read schema version: %w", err)
	}
	if version > CurrentServerSchemaVersion {
		return fmt.Errorf("server store: schema version %d is newer than supported version %d", version, CurrentServerSchemaVersion)
	}
	if version < 1 {
		if err := s.migrateTo1(); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		if err := s.migrateTo2(); err != nil {
			return err
		}
		version = 2
	}
	if version < 3 {
		if err := s.migrateTo3(); err != nil {
			return err
		}
		version = 3
	}
	if version < 4 {
		if err := s.migrateTo4(); err != nil {
			return err
		}
		version = 4
	}
	if version < 5 {
		if err := s.migrateTo5(); err != nil {
			return err
		}
		version = 5
	}
	if version < 6 {
		if err := s.migrateTo6(); err != nil {
			return err
		}
		version = 6
	}
	if version < 7 {
		if err := s.migrateTo7(); err != nil {
			return err
		}
		version = 7
	}
	if version < 8 {
		if err := s.migrateTo8(); err != nil {
			return err
		}
		version = 8
	}
	if version < 9 {
		if err := s.migrateTo9(); err != nil {
			return err
		}
		version = 9
	}
	if version < 10 {
		if err := s.migrateTo10(); err != nil {
			return err
		}
		version = 10
	}
	if version < 11 {
		if err := s.migrateTo11(); err != nil {
			return err
		}
		version = 11
	}
	if version < 12 {
		if err := s.migrateTo12(); err != nil {
			return err
		}
		version = 12
	}
	if _, err := s.db.Exec(`INSERT INTO schema_meta(id, version) VALUES(1, ?) ON CONFLICT(id) DO UPDATE SET version = excluded.version`, version); err != nil {
		return fmt.Errorf("server store: write schema version: %w", err)
	}
	return nil
}

func (s *Store) migrateTo1() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(serverSchemaV1); err != nil {
		return fmt.Errorf("server store: migrate v1: %w", err)
	}
	return tx.Commit()
}

func (s *Store) migrateTo2() error {
	if _, err := s.db.Exec(serverSchemaV2); err != nil {
		return fmt.Errorf("server store: migrate v2: %w", err)
	}
	return nil
}

func (s *Store) migrateTo3() error {
	if _, err := s.db.Exec(serverSchemaV3); err != nil {
		return fmt.Errorf("server store: migrate v3: %w", err)
	}
	return nil
}

func (s *Store) migrateTo4() error {
	if _, err := s.db.Exec(serverSchemaV4); err != nil {
		return fmt.Errorf("server store: migrate v4: %w", err)
	}
	return nil
}

func (s *Store) migrateTo5() error {
	if _, err := s.db.Exec(serverSchemaV5); err != nil {
		return fmt.Errorf("server store: migrate v5: %w", err)
	}
	return nil
}

func (s *Store) migrateTo6() error {
	if _, err := s.db.Exec(serverSchemaV6); err != nil {
		return fmt.Errorf("server store: migrate v6: %w", err)
	}
	return nil
}

func (s *Store) migrateTo7() error {
	// ALTER TABLE ADD COLUMN is not idempotent, and a re-run migration (e.g. after a
	// forced version reset) must not fail on an already-present column. Guard on the
	// column's existence via table_info.
	has, err := s.columnExists("users", "suspended_at")
	if err != nil {
		return fmt.Errorf("server store: migrate v7: %w", err)
	}
	if has {
		return nil
	}
	if _, err := s.db.Exec(serverSchemaV7); err != nil {
		return fmt.Errorf("server store: migrate v7: %w", err)
	}
	return nil
}

// migrateTo8 backs the Custom Sync identity core (TODOS.md C418-C422): a
// device_label on each refresh-token family so the device/session list has a
// human-readable name, and phone_number/password_hash on users so SMS and
// username/password enrollment extend the SAME users table rather than a
// parallel identity table (matching the existing "provider:subject" id
// convention — see internal/server/repository.go User). Every ALTER is
// guarded by columnExists because ALTER TABLE ADD COLUMN is not idempotent
// and a re-run migration must not fail on an already-present column.
func (s *Store) migrateTo8() error {
	if has, err := s.columnExists("refresh_tokens", "device_label"); err != nil {
		return fmt.Errorf("server store: migrate v8: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE refresh_tokens ADD COLUMN device_label TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v8: %w", err)
		}
	}
	if has, err := s.columnExists("users", "phone_number"); err != nil {
		return fmt.Errorf("server store: migrate v8: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN phone_number TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v8: %w", err)
		}
	}
	if has, err := s.columnExists("users", "password_hash"); err != nil {
		return fmt.Errorf("server store: migrate v8: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v8: %w", err)
		}
	}
	// A partial unique index (rather than a column-level UNIQUE, which SQLite's
	// ALTER TABLE ADD COLUMN cannot express) so any number of users with no
	// phone number yet ('') coexist, while two users can never claim the same
	// verified phone number.
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_number ON users(phone_number) WHERE phone_number != '';`); err != nil {
		return fmt.Errorf("server store: migrate v8: %w", err)
	}
	return nil
}

// migrateTo9 adds subscriptions.last_event_at (TODOS.md C430): the
// provider-supplied event timestamp of the most recent webhook that mutated
// this subscription's status. Webhook delivery is at-least-once but not
// ordered — a delayed "past_due" retry can otherwise arrive after a newer
// "canceled" event and silently un-cancel the row. Guarding every
// status-affecting apply with "only write if this event is newer than what's
// already there" needs a durable per-subscription high-water mark, hence the
// column (in-memory tracking would not survive a restart or a multi-instance
// deployment).
func (s *Store) migrateTo9() error {
	if has, err := s.columnExists("subscriptions", "last_event_at"); err != nil {
		return fmt.Errorf("server store: migrate v9: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE subscriptions ADD COLUMN last_event_at TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v9: %w", err)
		}
	}
	return nil
}

// migrateTo10 adds the pairing_codes table backing AuthService.RedeemPairingCode
// (TODOS.md C421): a short-lived, single-use code the portal mints for an
// existing account so a new device can link to it without re-entering a
// password. A dedicated table (rather than reusing refresh_tokens/users) keeps
// the code's short TTL and single-use consumption independent of session
// lifecycle, and lets an unconsumed, expired code simply age out.
//
// It also adds users.recovery_code_hash (TODOS.md C422): a bcrypt hash of the
// one-time account-recovery code shown to a Register caller exactly once.
// Password accounts have no email/SMS-backed recovery path, so this is the
// only thing standing between a forgotten password and a locked-out account;
// storing only the hash (never the plaintext) means a later ResetPassword
// ticket can verify a presented recovery code without this column itself
// being a second password.
func (s *Store) migrateTo10() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS pairing_codes (
	code TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	consumed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pairing_codes_user_id ON pairing_codes(user_id);
`); err != nil {
		return fmt.Errorf("server store: migrate v10: %w", err)
	}
	if has, err := s.columnExists("users", "recovery_code_hash"); err != nil {
		return fmt.Errorf("server store: migrate v10: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN recovery_code_hash TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v10: %w", err)
		}
	}
	return nil
}

// migrateTo11 originally added the setup_codes table backing a phone/SMS
// enrollment gate (Config.SetupCode) and users.phone_verified_at. Both are now
// vestigial: phone/SMS sign-in was removed entirely (replaced by an
// admin-approved device-pairing bootstrap), so nothing reads or writes either
// anymore. Left in place rather than migrated away — a SQLite column/table
// drop isn't worth the risk for schema nothing references.
func (s *Store) migrateTo11() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS setup_codes (
	code_hash TEXT PRIMARY KEY,
	consumed_at TEXT NOT NULL DEFAULT ''
);
`); err != nil {
		return fmt.Errorf("server store: migrate v11: %w", err)
	}
	if has, err := s.columnExists("users", "phone_verified_at"); err != nil {
		return fmt.Errorf("server store: migrate v11: %w", err)
	} else if !has {
		if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN phone_verified_at TEXT NOT NULL DEFAULT '';`); err != nil {
			return fmt.Errorf("server store: migrate v11: %w", err)
		}
	}
	return nil
}

// migrateTo12 originally added the invite_codes table backing admin-mintable
// single-use phone-enrollment invites — a companion to the now-removed
// setup_codes gate. Vestigial along with it (see migrateTo11's doc comment):
// nothing mints, reads, or consumes rows here anymore. Left in place rather
// than migrated away.
func (s *Store) migrateTo12() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS invite_codes (
	code TEXT PRIMARY KEY,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	consumed_at TEXT NOT NULL DEFAULT ''
);
`); err != nil {
		return fmt.Errorf("server store: migrate v12: %w", err)
	}
	return nil
}

// columnExists reports whether a table has a column of the given name.
func (s *Store) columnExists(table, column string) (bool, error) {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

const serverSchemaV1 = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  subject TEXT NOT NULL,
  email TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(provider, subject)
);

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  color TEXT NOT NULL DEFAULT '',
  sort INTEGER NOT NULL DEFAULT 0,
  deleted INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  device_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_workspaces_user ON workspaces(user_id, deleted, sort);

CREATE TABLE IF NOT EXISTS snapshots (
  workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  dataset_json BLOB NOT NULL,
  version INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshot_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  dataset_json BLOB NOT NULL,
  version INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshot_history_workspace ON snapshot_history(workspace_id, version DESC);

CREATE TABLE IF NOT EXISTS blobs (
  hash TEXT PRIMARY KEY,
  size INTEGER NOT NULL,
  mime TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workspace_blobs (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  hash TEXT NOT NULL REFERENCES blobs(hash) ON DELETE CASCADE,
  PRIMARY KEY(workspace_id, hash)
);

CREATE TABLE IF NOT EXISTS ai_keys (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  ciphertext BLOB NOT NULL,
  nonce BLOB NOT NULL,
  PRIMARY KEY(user_id, provider)
);

CREATE TABLE IF NOT EXISTS usage (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  day TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  tokens INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(user_id, day)
);
`

const serverSchemaV2 = `
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  request_id TEXT NOT NULL,
  previous_hash TEXT NOT NULL,
  hash TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_events_id ON audit_events(id);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events(actor_id, id);
`

const serverSchemaV3 = `
CREATE TABLE IF NOT EXISTS refresh_tokens (
  jti TEXT PRIMARY KEY,
  family_id TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  used_at TEXT NOT NULL DEFAULT '',
  revoked_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
`

const serverSchemaV4 = `
CREATE TABLE IF NOT EXISTS subscriptions (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  stripe_customer TEXT NOT NULL,
  stripe_subscription TEXT NOT NULL,
  status TEXT NOT NULL,
  plan TEXT NOT NULL,
  current_period_end TEXT NOT NULL,
  trial_end TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(stripe_customer),
  UNIQUE(stripe_subscription)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
`

const serverSchemaV5 = `
CREATE TABLE IF NOT EXISTS idempotency_keys (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  route TEXT NOT NULL,
  key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  response_body BLOB NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(user_id, route, key)
);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created ON idempotency_keys(created_at);
`

// serverSchemaV6 generalizes the subscriptions table for multiple payment
// providers (Stripe + PayPal): a provider discriminator column and
// provider-neutral customer/subscription id columns, with the uniqueness moved
// to (provider, provider_subscription). A single-column UNIQUE can't be altered
// in place, so the table is rebuilt and existing rows are migrated as 'stripe'.
// Also adds a webhook-event dedupe table so replayed provider webhooks are
// applied at most once.
const serverSchemaV6 = `
CREATE TABLE subscriptions_v6 (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL DEFAULT 'stripe',
  provider_customer TEXT NOT NULL,
  provider_subscription TEXT NOT NULL,
  status TEXT NOT NULL,
  plan TEXT NOT NULL,
  current_period_end TEXT NOT NULL,
  trial_end TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(provider, provider_customer),
  UNIQUE(provider, provider_subscription)
);
INSERT INTO subscriptions_v6(user_id, provider, provider_customer, provider_subscription, status, plan, current_period_end, trial_end, updated_at)
  SELECT user_id, 'stripe', stripe_customer, stripe_subscription, status, plan, current_period_end, trial_end, updated_at FROM subscriptions;
DROP TABLE subscriptions;
ALTER TABLE subscriptions_v6 RENAME TO subscriptions;
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);

CREATE TABLE IF NOT EXISTS webhook_events (
  provider TEXT NOT NULL,
  event_id TEXT NOT NULL,
  received_at TEXT NOT NULL,
  PRIMARY KEY(provider, event_id)
);
CREATE INDEX IF NOT EXISTS idx_webhook_events_received ON webhook_events(received_at);
`

// serverSchemaV7 adds an operator suspension marker to users. NULL means active;
// a timestamp means the account is suspended (as of that time). Suspension blocks
// new OAuth sessions and denies the cloud entitlement without deleting any data.
const serverSchemaV7 = `
ALTER TABLE users ADD COLUMN suspended_at TEXT NOT NULL DEFAULT '';
`
