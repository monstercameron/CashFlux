package server

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestOpenStoreMigratesSchema(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	version, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentServerSchemaVersion)
	}
	for _, table := range []string{
		"users", "workspaces", "snapshots", "snapshot_history", "blobs",
		"workspace_blobs", "ai_keys", "usage", "audit_events", "refresh_tokens", "subscriptions",
	} {
		if !tableExists(t, s.db, table) {
			t.Fatalf("missing table %s", table)
		}
	}
}

func TestOpenStoreAppliesSQLiteTuning(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	stats := s.db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}
	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	var busyTimeout int
	if err := s.db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busyTimeout != sqliteBusyTimeoutMillis {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMillis)
	}
}

func TestOpenStoreRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cashflux.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
INSERT INTO schema_meta(id, version) VALUES(1, 99);`); err != nil {
		t.Fatalf("seed newer schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	s, err := OpenStore(path)
	if err == nil {
		_ = s.Close()
		t.Fatal("OpenStore accepted a newer schema")
	}
}

func TestStoreReady(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Ready(); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Ready(); err == nil {
		t.Fatal("Ready succeeded after Close")
	}
}

func TestStoreCheckpointWAL(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: time.Date(2026, time.June, 18, 23, 20, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.CheckpointWAL(context.Background()); err != nil {
		t.Fatalf("CheckpointWAL: %v", err)
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query table %s: %v", name, err)
	}
	return got == name
}
