package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver" // registers the pure-Go sqlite3 driver
)

const CurrentServerSchemaVersion = 1

// Store owns the backend SQLite database.
type Store struct {
	db *sql.DB
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
	if _, err := db.Exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;"); err != nil {
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

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// CheckpointWAL flushes the SQLite write-ahead log back into the main database file.
func (s *Store) CheckpointWAL(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
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
