// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// Schema v17 rebuilds four tables on a live database. These tests build a v16
// database by hand, migrate it, and check that nothing was lost — the only part
// of this change that cannot be undone by shipping a fix afterwards.

// v16Schema is the shape of the tables as they stood before v17: workspaces
// keyed globally by id, children keyed by workspace_id alone.
const v16Schema = `
-- users as it stands at v16: the base columns plus everything earlier
-- migrations bolted on with ALTER. Spelled out because these fixtures stamp
-- straight to v16, so steps 1-16 never run against them.
CREATE TABLE users (
  id TEXT PRIMARY KEY, provider TEXT NOT NULL, subject TEXT NOT NULL,
  email TEXT NOT NULL, created_at TEXT NOT NULL,
  phone_number TEXT NOT NULL DEFAULT '',
  phone_verified_at TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  recovery_code_hash TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT 'member',
  suspended_at TEXT NOT NULL DEFAULT '',
  UNIQUE(provider, subject)
);
CREATE TABLE workspaces (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL, color TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0,
  deleted INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL, device_id TEXT NOT NULL DEFAULT ''
);
CREATE TABLE snapshots (
  workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  dataset_json BLOB NOT NULL, version INTEGER NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE snapshot_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  dataset_json BLOB NOT NULL, version INTEGER NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE blobs (
  hash TEXT PRIMARY KEY, size INTEGER NOT NULL, mime TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE workspace_blobs (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  hash TEXT NOT NULL REFERENCES blobs(hash) ON DELETE CASCADE,
  PRIMARY KEY(workspace_id, hash)
);
-- Added by v16; present in any real database that reaches v17.
CREATE TABLE deleted_accounts (user_id TEXT PRIMARY KEY, deleted_at TEXT NOT NULL);
CREATE TABLE server_secrets (name TEXT PRIMARY KEY, secret TEXT NOT NULL, created_at TEXT NOT NULL);
`

// buildV16Database writes a pre-v17 database to disk and returns its path.
func buildV16Database(t *testing.T, seed func(*sql.DB)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v16.db")
	db, err := sql.Open("sqlite3", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(v16Schema); err != nil {
		t.Fatalf("v16 schema: %v", err)
	}
	// Stamp it as v16 so migrate() runs exactly the one step under test.
	if _, err := db.Exec(`CREATE TABLE schema_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL)`); err != nil {
		t.Fatalf("schema_meta: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_meta(id, version) VALUES(1, 16)`); err != nil {
		t.Fatalf("stamp version: %v", err)
	}
	seed(db)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return path
}

func TestMigrateV17PreservesEveryRow(t *testing.T) {
	now := formatTime(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	path := buildV16Database(t, func(db *sql.DB) {
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.Exec(q, args...); err != nil {
				t.Fatalf("seed %q: %v", q, err)
			}
		}
		for _, u := range []string{"u1", "u2"} {
			exec(`INSERT INTO users(id, provider, subject, email, created_at) VALUES(?, 'github', ?, '', ?)`, u, u, now)
		}
		// Ids are globally unique before v17, so each maps to exactly one owner.
		exec(`INSERT INTO workspaces(id, user_id, name, updated_at, version) VALUES('default', 'u1', 'Home', ?, 3)`, now)
		exec(`INSERT INTO workspaces(id, user_id, name, updated_at, version) VALUES('ws_travel', 'u1', 'Travel', ?, 1)`, now)
		exec(`INSERT INTO workspaces(id, user_id, name, updated_at, version) VALUES('ws_other', 'u2', 'Other', ?, 2)`, now)

		exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES('default', ?, 3, ?)`, []byte(`{"owner":"u1"}`), now)
		exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES('ws_other', ?, 2, ?)`, []byte(`{"owner":"u2"}`), now)
		exec(`INSERT INTO snapshot_history(workspace_id, dataset_json, version, updated_at) VALUES('default', ?, 1, ?)`, []byte(`{"v":1}`), now)
		exec(`INSERT INTO snapshot_history(workspace_id, dataset_json, version, updated_at) VALUES('default', ?, 2, ?)`, []byte(`{"v":2}`), now)

		exec(`INSERT INTO blobs(hash, size, mime, created_at) VALUES('h1', 7, 'text/plain', ?)`, now)
		exec(`INSERT INTO workspace_blobs(workspace_id, hash) VALUES('default', 'h1')`)
		exec(`INSERT INTO workspace_blobs(workspace_id, hash) VALUES('ws_other', 'h1')`)
	})

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore (runs the migration): %v", err)
	}
	defer func() { _ = store.Close() }()

	// Workspaces keep their owners.
	for _, tc := range []struct{ user, ws, name string }{
		{"u1", "default", "Home"},
		{"u1", "ws_travel", "Travel"},
		{"u2", "ws_other", "Other"},
	} {
		got, ok, err := store.GetWorkspace(tc.user, tc.ws)
		if err != nil || !ok {
			t.Fatalf("GetWorkspace(%s, %s): ok=%v err=%v", tc.user, tc.ws, ok, err)
		}
		if got.Name != tc.name {
			t.Fatalf("workspace %s name = %q, want %q", tc.ws, got.Name, tc.name)
		}
	}

	// Snapshots landed under the right account.
	for _, tc := range []struct{ user, ws, want string }{
		{"u1", "default", `{"owner":"u1"}`},
		{"u2", "ws_other", `{"owner":"u2"}`},
	} {
		snap, ok, err := store.GetSnapshotForUser(tc.user, tc.ws)
		if err != nil || !ok {
			t.Fatalf("snapshot(%s, %s): ok=%v err=%v", tc.user, tc.ws, ok, err)
		}
		if string(snap.Dataset) != tc.want {
			t.Fatalf("snapshot(%s) = %s, want %s", tc.ws, snap.Dataset, tc.want)
		}
	}
	// ...and cannot be read by the other account.
	if _, ok, _ := store.GetSnapshotForUser("u2", "default"); ok {
		t.Fatal("u2 can read u1's migrated snapshot")
	}

	history, err := store.SnapshotHistory("u1", "default", 0)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history rows = %d, want 2 — the migration dropped archived versions", len(history))
	}

	for _, tc := range []struct {
		user, ws string
		want     bool
	}{
		{"u1", "default", true},
		{"u2", "ws_other", true},
		{"u2", "default", false}, // never had this link
	} {
		linked, err := store.UserWorkspaceBlob(tc.user, tc.ws, "h1")
		if err != nil {
			t.Fatalf("UserWorkspaceBlob(%s, %s): %v", tc.user, tc.ws, err)
		}
		if linked != tc.want {
			t.Fatalf("blob link (%s, %s) = %v, want %v", tc.user, tc.ws, linked, tc.want)
		}
	}
}

// After migrating, the collision the whole change exists to fix must be gone.
func TestMigrateV17UnblocksASecondAccount(t *testing.T) {
	now := formatTime(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	path := buildV16Database(t, func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO users(id, provider, subject, email, created_at) VALUES('u1','github','u1','',?)`, now); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO workspaces(id, user_id, name, updated_at) VALUES('default','u1','Home',?)`, now); err != nil {
			t.Fatalf("seed workspace: %v", err)
		}
	})

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	svc := NewSyncService(store)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u2"})
	stamp := time.Date(2026, 8, 18, 12, 5, 0, 0, time.UTC)
	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "default", Name: "Default"}, stamp, false, stamp); err != nil {
		t.Fatalf("a second account is still blocked after migrating: %v", err)
	}
}

// Running the migration twice must be a no-op, not a second rebuild — servers
// restart, and a migration that only works once is a migration that eventually
// destroys something.
func TestMigrateV17IsIdempotent(t *testing.T) {
	now := formatTime(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	path := buildV16Database(t, func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO users(id, provider, subject, email, created_at) VALUES('u1','github','u1','',?)`, now); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO workspaces(id, user_id, name, updated_at) VALUES('default','u1','Home',?)`, now); err != nil {
			t.Fatalf("seed workspace: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES('default',?,1,?)`, []byte(`{"keep":true}`), now); err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
	})

	for pass := 1; pass <= 3; pass++ {
		store, err := OpenStore(path)
		if err != nil {
			t.Fatalf("open pass %d: %v", pass, err)
		}
		snap, ok, err := store.GetSnapshotForUser("u1", "default")
		if err != nil || !ok || string(snap.Dataset) != `{"keep":true}` {
			t.Fatalf("pass %d: snapshot = %s ok=%v err=%v", pass, snap.Dataset, ok, err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("close pass %d: %v", pass, err)
		}
	}
}

// A child row whose workspace has already vanished is an orphan a foreign key
// would have removed. It must not fail the migration for everybody else.
func TestMigrateV17SkipsOrphanedChildRows(t *testing.T) {
	now := formatTime(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	path := buildV16Database(t, func(db *sql.DB) {
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.Exec(q, args...); err != nil {
				t.Fatalf("seed %q: %v", q, err)
			}
		}
		exec(`PRAGMA foreign_keys = OFF`)
		exec(`INSERT INTO users(id, provider, subject, email, created_at) VALUES('u1','github','u1','',?)`, now)
		exec(`INSERT INTO workspaces(id, user_id, name, updated_at) VALUES('default','u1','Home',?)`, now)
		exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES('default',?,1,?)`, []byte(`{"real":true}`), now)
		// No workspace row for this one.
		exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES('ghost',?,1,?)`, []byte(`{"orphan":true}`), now)
		exec(`INSERT INTO snapshot_history(workspace_id, dataset_json, version, updated_at) VALUES('ghost',?,1,?)`, []byte(`{}`), now)
	})

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("an orphaned child row broke the migration: %v", err)
	}
	defer func() { _ = store.Close() }()

	snap, ok, err := store.GetSnapshotForUser("u1", "default")
	if err != nil || !ok || string(snap.Dataset) != `{"real":true}` {
		t.Fatalf("the real snapshot did not survive: %s ok=%v err=%v", snap.Dataset, ok, err)
	}
	var ghosts int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE workspace_id = 'ghost'`).Scan(&ghosts); err != nil {
		t.Fatalf("count ghosts: %v", err)
	}
	if ghosts != 0 {
		t.Fatalf("orphaned rows were carried into the scoped table: %d", ghosts)
	}
}
