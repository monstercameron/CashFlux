// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// seedDataset is a dataset with something in every shape the round trip has to
// preserve: entities, nested values, app state, settings, and unicode.
func seedDataset() Dataset {
	return Dataset{
		Members: []domain.Member{{ID: "m1", Name: "Cam"}, {ID: "m2", Name: "Zoë"}},
		Accounts: []domain.Account{
			{ID: "a1", Name: "Checking", Currency: "USD"},
			{ID: "a2", Name: "Crédit — carte", Currency: "EUR"},
		},
		Categories: []domain.Category{{ID: "c1", Name: "Groceries"}},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "a1", CategoryID: "c1", Amount: money.New(-1250, "USD"), Desc: "Milk, eggs"},
			{ID: "t2", AccountID: "a1", Amount: money.New(500000, "USD"), Desc: "Payday 💸"},
		},
		KV:         map[string]string{"layout": `{"tiles":["a","b"]}`},
		SettingsKV: map[string]string{"theme": "dark"},
		Settings:   Settings{BaseCurrency: "USD"},
	}
}

func loadedStore(t *testing.T, ds Dataset) *SQLiteStore {
	t.Helper()
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Load(ds); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

// The headline promise: what comes out is a real SQLite database file, and what
// goes back in is the same data.
func TestSQLiteFileRoundTrip(t *testing.T) {
	original := seedDataset()
	s := loadedStore(t, original)

	data, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("SerializeSQLite: %v", err)
	}
	if !LooksLikeSQLiteFile(data) {
		t.Fatalf("exported bytes do not start with the SQLite magic: %q", firstBytes(data, 16))
	}

	restored, err := ImportSQLiteFile(data)
	if err != nil {
		t.Fatalf("ImportSQLiteFile: %v", err)
	}

	if got, want := len(restored.Members), len(original.Members); got != want {
		t.Fatalf("members = %d, want %d", got, want)
	}
	if got, want := len(restored.Transactions), len(original.Transactions); got != want {
		t.Fatalf("transactions = %d, want %d", got, want)
	}
	if restored.Members[1].Name != "Zoë" {
		t.Fatalf("unicode member name did not survive: %q", restored.Members[1].Name)
	}
	if restored.Transactions[1].Desc != "Payday 💸" {
		t.Fatalf("unicode description did not survive: %q", restored.Transactions[1].Desc)
	}
	if restored.Transactions[0].Amount.Amount != -1250 {
		t.Fatalf("negative minor amount = %d, want -1250", restored.Transactions[0].Amount.Amount)
	}
	if restored.Accounts[1].Currency != "EUR" {
		t.Fatalf("second account currency = %q, want EUR", restored.Accounts[1].Currency)
	}
	if restored.KV["layout"] != original.KV["layout"] {
		t.Fatalf("app state did not survive: %q", restored.KV["layout"])
	}
	if restored.SettingsKV["theme"] != "dark" {
		t.Fatalf("settings state did not survive: %q", restored.SettingsKV["theme"])
	}
	if restored.Settings.BaseCurrency != "USD" {
		t.Fatalf("settings did not survive: %+v", restored.Settings)
	}
	if restored.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", restored.SchemaVersion, SchemaVersion)
	}
}

// Exporting, importing, and exporting again must produce the same dataset —
// the property that makes the file safe to move between machines repeatedly.
func TestSQLiteFileRoundTripIsStable(t *testing.T) {
	first := loadedStore(t, seedDataset())
	dataA, err := first.SerializeSQLite()
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	dsA, err := ImportSQLiteFile(dataA)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}

	second := loadedStore(t, dsA)
	dataB, err := second.SerializeSQLite()
	if err != nil {
		t.Fatalf("second export: %v", err)
	}
	dsB, err := ImportSQLiteFile(dataB)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}

	jsonA, err := Export(dsA)
	if err != nil {
		t.Fatalf("export A: %v", err)
	}
	jsonB, err := Export(dsB)
	if err != nil {
		t.Fatalf("export B: %v", err)
	}
	if string(jsonA) != string(jsonB) {
		t.Fatal("a second round trip changed the dataset")
	}
}

// The exported file has to be a database any SQLite tool can open and query,
// not merely something this package can read back.
func TestExportedFileIsQueryableAsAnOrdinaryDatabase(t *testing.T) {
	s := loadedStore(t, seedDataset())
	data, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("SerializeSQLite: %v", err)
	}

	// Open the bytes the way an unrelated tool would: a fresh connection that
	// knows nothing about this package's types.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()
	if err := deserializeInto(db, "main", data); err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&count); err != nil {
		t.Fatalf("plain SQL against the exported file: %v", err)
	}
	if count != 2 {
		t.Fatalf("transactions rows = %d, want 2", count)
	}
	var name string
	if err := db.QueryRow(`SELECT json_extract(data, '$.name') FROM accounts WHERE id = 'a1'`).Scan(&name); err != nil {
		t.Fatalf("json_extract against the exported file: %v", err)
	}
	if name != "Checking" {
		t.Fatalf("account name via SQL = %q, want Checking", name)
	}
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != SchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, SchemaVersion)
	}
}

// Anything that is not one of our databases must be refused with the live store
// untouched — the import reads into a scratch database precisely so a bad file
// cannot half-apply.
func TestImportSQLiteFileRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ErrNotSQLiteFile},
		{"too short for the magic", []byte("SQLite"), ErrNotSQLiteFile},
		{"JSON, not a database", []byte(`{"members":[],"schemaVersion":1}`), ErrNotSQLiteFile},
		{"a CSV export", []byte("date,amount,description\n2026-01-01,10,Coffee\n"), ErrNotSQLiteFile},
		{"magic but truncated", []byte(sqliteFileHeader + "\x00\x00"), ErrNotSQLiteFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ImportSQLiteFile(tc.data)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// A real SQLite database from some other program opens fine and is empty of our
// tables. Accepting it would look like a successful import that erased
// everything, so it is refused by name.
func TestImportSQLiteFileRejectsAForeignDatabase(t *testing.T) {
	foreign, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	foreign.SetMaxOpenConns(1)
	defer func() { _ = foreign.Close() }()
	if _, err := foreign.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)`); err != nil {
		t.Fatalf("seed foreign db: %v", err)
	}
	if _, err := foreign.Exec(`INSERT INTO notes(body) VALUES('someone else data')`); err != nil {
		t.Fatalf("seed foreign row: %v", err)
	}
	data, err := serializeConn(foreign, "main")
	if err != nil {
		t.Fatalf("serialize foreign db: %v", err)
	}

	_, err = ImportSQLiteFile(data)
	if !errors.Is(err, ErrNotCashFluxDatabase) {
		t.Fatalf("err = %v, want ErrNotCashFluxDatabase", err)
	}
	// The message should name what was missing, so the person can tell which
	// kind of wrong file they picked.
	if !strings.Contains(err.Error(), "members") {
		t.Fatalf("error does not say what was missing: %v", err)
	}
}

// A database missing even one of our tables is not importable — a partially
// matching file is the one most likely to be accepted and then behave oddly.
func TestImportSQLiteFileRejectsAPartialSchema(t *testing.T) {
	partial, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	partial.SetMaxOpenConns(1)
	defer func() { _ = partial.Close() }()

	// Everything except transactions.
	for _, name := range schemaTableNames() {
		if name == "transactions" {
			continue
		}
		if _, err := partial.Exec("CREATE TABLE " + name + " (id TEXT PRIMARY KEY, data TEXT NOT NULL)"); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	data, err := serializeConn(partial, "main")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	_, err = ImportSQLiteFile(data)
	if !errors.Is(err, ErrNotCashFluxDatabase) {
		t.Fatalf("err = %v, want ErrNotCashFluxDatabase", err)
	}
	if !strings.Contains(err.Error(), "transactions") {
		t.Fatalf("error should name the missing table: %v", err)
	}
}

// A file from a future build must be refused rather than half-understood, the
// same rule Import applies to JSON.
func TestImportSQLiteFileRejectsANewerSchema(t *testing.T) {
	s := loadedStore(t, seedDataset())
	if _, err := s.db.Exec("PRAGMA user_version = 9999"); err != nil {
		t.Fatalf("stamp future version: %v", err)
	}
	// Serialize directly so SerializeSQLite does not re-stamp the version.
	data, err := serializeConn(s.db, "main")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if _, err := ImportSQLiteFile(data); err == nil {
		t.Fatal("a newer-than-supported database was accepted")
	} else if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("err = %v, want a newer-than-supported error", err)
	}
}

// A file written before user_version was stamped reads as 0, which migrate
// treats as the initial release rather than as a corrupt file.
func TestImportSQLiteFileAcceptsAnUnstampedDatabase(t *testing.T) {
	s := loadedStore(t, seedDataset())
	if _, err := s.db.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatalf("clear version: %v", err)
	}
	data, err := serializeConn(s.db, "main")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	ds, err := ImportSQLiteFile(data)
	if err != nil {
		t.Fatalf("unstamped database was refused: %v", err)
	}
	if ds.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want it migrated to %d", ds.SchemaVersion, SchemaVersion)
	}
	if len(ds.Transactions) != 2 {
		t.Fatalf("transactions = %d, want 2", len(ds.Transactions))
	}
}

// An empty household exports and imports like any other — the case a new user
// hits first, and the one where an "is there anything here" shortcut would bite.
func TestSQLiteFileRoundTripOfAnEmptyDataset(t *testing.T) {
	s := loadedStore(t, EmptyDataset())
	data, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("SerializeSQLite: %v", err)
	}
	ds, err := ImportSQLiteFile(data)
	if err != nil {
		t.Fatalf("ImportSQLiteFile: %v", err)
	}
	if len(ds.Transactions) != 0 || len(ds.Accounts) != 0 {
		t.Fatalf("expected an empty dataset, got %d accounts and %d transactions", len(ds.Accounts), len(ds.Transactions))
	}
}

// The vacuum is what keeps the exported file proportional to what the user has
// rather than to everything they have ever deleted.
func TestSerializeSQLiteCompactsDeletedData(t *testing.T) {
	big := seedDataset()
	for i := 0; i < 4000; i++ {
		big.Transactions = append(big.Transactions, domain.Transaction{
			ID: "bulk" + itoa(i), AccountID: "a1", Amount: money.New(int64(i), "USD"),
			Desc: strings.Repeat("padding ", 16),
		})
	}
	s := loadedStore(t, big)
	full, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("export with data: %v", err)
	}

	// Replace the contents with the small dataset; the freed pages must not
	// travel with the export.
	if err := s.Load(seedDataset()); err != nil {
		t.Fatalf("reload small: %v", err)
	}
	small, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("export after shrink: %v", err)
	}
	if len(small) >= len(full) {
		t.Fatalf("export did not shrink after deleting data: %d bytes before, %d after", len(full), len(small))
	}
	// And it still reads back correctly.
	ds, err := ImportSQLiteFile(small)
	if err != nil {
		t.Fatalf("import compacted file: %v", err)
	}
	if len(ds.Transactions) != 2 {
		t.Fatalf("transactions after compaction = %d, want 2", len(ds.Transactions))
	}
}

// schemaTableNames is what the import validates against, so it must actually
// agree with the schema the store creates.
func TestSchemaTableNamesMatchTheLiveSchema(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer func() { _ = s.Close() }()

	live, err := tableNames(s.db)
	if err != nil {
		t.Fatalf("tableNames: %v", err)
	}
	parsed := schemaTableNames()
	if len(parsed) == 0 {
		t.Fatal("schemaTableNames parsed nothing out of sqliteSchema")
	}
	for _, name := range parsed {
		if !live[name] {
			t.Fatalf("schemaTableNames lists %q, which the store does not create", name)
		}
	}
	for name := range live {
		if strings.HasPrefix(name, "sqlite_") {
			continue // SQLite's own bookkeeping tables
		}
		if !contains(parsed, name) {
			t.Fatalf("the store creates table %q, which schemaTableNames misses — an import would not check it", name)
		}
	}
	if missing := missingTables(live); len(missing) != 0 {
		t.Fatalf("a freshly created store reports missing tables: %v", missing)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func firstBytes(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return string(b[:n])
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
