// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
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

// TestStoreMigrateIsIdempotent proves that re-running migrate() against an
// already-fully-migrated database (v8/v9/v10's new columns/tables/indexes
// included) is a clean no-op: no error, no duplicate-column/duplicate-index
// failure, and the schema version is unchanged.
func TestStoreMigrateIsIdempotent(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate() call: %v", err)
	}
	if err := s.migrate(); err != nil {
		t.Fatalf("third migrate() call: %v", err)
	}
	version, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("schema version after repeated migrate() = %d, want %d", version, CurrentServerSchemaVersion)
	}
}

// TestMigrateV8ThroughV10ColumnsAndTablesExist proves the v8/v9/v10 migrations
// (added across separate lanes) landed without colliding: every column/table/
// index each one adds is present exactly once on a freshly opened store.
func TestMigrateV8ThroughV10ColumnsAndTablesExist(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	for _, tc := range []struct{ table, column string }{
		{"refresh_tokens", "device_label"},
		{"users", "phone_number"},
		{"users", "password_hash"},
		{"subscriptions", "last_event_at"},
		{"users", "recovery_code_hash"},
	} {
		has, err := s.columnExists(tc.table, tc.column)
		if err != nil {
			t.Fatalf("columnExists(%s, %s): %v", tc.table, tc.column, err)
		}
		if !has {
			t.Fatalf("missing column %s.%s", tc.table, tc.column)
		}
	}
	if !tableExists(t, s.db, "pairing_codes") {
		t.Fatal("missing table pairing_codes")
	}
}

// TestPhoneNumberColumnIsNeverPopulated documents a gap between migrateTo8's
// doc comment and what the codebase actually does: the comment says the
// partial unique index on users.phone_number exists so "two users can never
// claim the same verified phone number", but no code path anywhere writes to
// that column — phone-based accounts are created via
// authservice.go's ensurePhoneUser/phoneUserID, which upserts a
// deterministic User{ID: "phone:"+phone, Provider: "phone", Subject: phone}
// row and relies on the users(provider, subject) UNIQUE constraint from
// serverSchemaV1 for de-duplication instead. That constraint does correctly
// prevent two distinct accounts for one phone number (both concurrent
// registrations resolve to the SAME deterministic id — see
// TestConcurrentPhoneVerificationResolvesToSameAccount in
// authservice_phone_test.go) but it means the phone_number column and its
// index added in migrateTo8 are dead: always empty, protecting nothing. This
// test pins that fact so it's caught if it silently starts to matter (e.g. a
// future feature that lets a password account link/claim a phone number by
// writing this column — at that point the uniqueness index is real and
// load-bearing, but until then it is inert).
func TestPhoneNumberColumnIsNeverPopulated(t *testing.T) {
	verify := newFakeVerifyClient()
	s := newPhoneTestAuthServer(t, verify)
	phone := "+15559990001"
	if _, err := s.RequestPhoneVerification(context.Background(), backendrpc.RequestPhoneVerificationRequest{PhoneNumber: phone}); err != nil {
		t.Fatalf("RequestPhoneVerification: %v", err)
	}
	if _, err := s.VerifyPhoneCode(context.Background(), backendrpc.VerifyPhoneCodeRequest{PhoneNumber: phone, Code: "123456"}); err != nil {
		t.Fatalf("VerifyPhoneCode: %v", err)
	}
	var phoneNumberColumn string
	if err := s.store.db.QueryRow(`SELECT phone_number FROM users WHERE id = ?`, "phone:"+phone).Scan(&phoneNumberColumn); err != nil {
		t.Fatalf("query phone_number column: %v", err)
	}
	if phoneNumberColumn != "" {
		t.Fatalf("phone_number column = %q, want empty — if this now fails, migrateTo8's partial unique index has become load-bearing and needs its own dedicated concurrency test", phoneNumberColumn)
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

// TestMigrateV5ToV6PreservesSubscriptions guards the commercial-critical property
// that upgrading the schema never loses a customer's subscription: it reconstructs
// a pre-v6 (stripe_*) subscriptions table with a row, rewinds the schema version,
// re-runs the migration, and asserts the row survived with provider "stripe" and
// the generalized ids intact.
func TestMigrateV5ToV6PreservesSubscriptions(t *testing.T) {
	s := openTestStore(t) // already migrated to the current version
	if _, err := s.db.Exec(`
DROP TABLE subscriptions;
CREATE TABLE subscriptions (
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
INSERT INTO users(id, provider, subject, email, created_at) VALUES('u1','token','u1','', '2026-01-01T00:00:00Z');
INSERT INTO subscriptions VALUES('u1','cus_old','sub_old','active','personal_monthly','','', '2026-01-01T00:00:00Z');
UPDATE schema_meta SET version = 5;`); err != nil {
		t.Fatalf("stage v5 fixture: %v", err)
	}
	if err := s.migrate(); err != nil {
		t.Fatalf("re-migrate to v6: %v", err)
	}
	got, ok, err := s.GetSubscription("u1")
	if err != nil || !ok {
		t.Fatalf("subscription lost after migration: ok=%v err=%v", ok, err)
	}
	if got.Provider != "stripe" || got.ProviderCustomer != "cus_old" || got.ProviderSubscription != "sub_old" || got.Plan != "personal_monthly" {
		t.Fatalf("migrated subscription = %+v", got)
	}
	// The generalized lookup finds it by (provider, subscription id).
	if _, ok, err := s.GetSubscriptionByProviderID("stripe", "sub_old"); err != nil || !ok {
		t.Fatalf("lookup by provider id after migration: ok=%v err=%v", ok, err)
	}
}
