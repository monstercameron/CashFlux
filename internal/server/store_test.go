// SPDX-License-Identifier: MIT

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
		"workspace_blobs", "ai_keys", "usage", "audit_events", "refresh_tokens", "subscriptions", "idempotency_keys",
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

func TestDryRunStoreMigrationsDoesNotMutateLiveDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cashflux.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);`); err != nil {
		t.Fatalf("seed schema meta: %v", err)
	}
	for _, stmt := range []string{serverSchemaV1, serverSchemaV2, serverSchemaV3} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO schema_meta(id, version) VALUES(1, 3);`); err != nil {
		t.Fatalf("seed schema version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	version, err := DryRunStoreMigrations(path)
	if err != nil {
		t.Fatalf("DryRunStoreMigrations: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("dry-run version = %d, want %d", version, CurrentServerSchemaVersion)
	}
	db, err = sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("reopen live db: %v", err)
	}
	defer db.Close()
	var liveVersion int
	if err := db.QueryRow("SELECT version FROM schema_meta WHERE id = 1").Scan(&liveVersion); err != nil {
		t.Fatalf("live schema version: %v", err)
	}
	if liveVersion != 3 {
		t.Fatalf("live schema version = %d, want unchanged 3", liveVersion)
	}
	if tableExists(t, db, "subscriptions") {
		t.Fatal("dry-run created subscriptions table in live database")
	}
	if tableExists(t, db, "idempotency_keys") {
		t.Fatal("dry-run created idempotency table in live database")
	}
}

func TestDryRunStoreMigrationsHandlesMissingDB(t *testing.T) {
	version, err := DryRunStoreMigrations(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("DryRunStoreMigrations missing db: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("dry-run missing db version = %d, want %d", version, CurrentServerSchemaVersion)
	}
}

func TestOpenStoreMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cashflux.db")
	first, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore first: %v", err)
	}
	if err := first.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: time.Date(2026, time.June, 19, 19, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}
	second, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore second: %v", err)
	}
	defer second.Close()
	version, err := second.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentServerSchemaVersion)
	}
	var schemaRows int
	if err := second.db.QueryRow("SELECT COUNT(*) FROM schema_meta").Scan(&schemaRows); err != nil {
		t.Fatalf("schema_meta count: %v", err)
	}
	if schemaRows != 1 {
		t.Fatalf("schema_meta rows = %d, want 1", schemaRows)
	}
	if _, ok, err := second.GetUserByID("u1"); err != nil || !ok {
		t.Fatalf("user after migration rerun = ok %v err %v", ok, err)
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
