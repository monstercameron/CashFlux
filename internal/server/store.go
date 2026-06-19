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

const CurrentServerSchemaVersion = 4
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
