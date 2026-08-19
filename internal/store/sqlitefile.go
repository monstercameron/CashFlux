// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	sqlitedriver "github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/serdes"
)

// Exporting and importing the database as an actual SQLite file.
//
// Every other export this app offers is JSON, and for good reason: the store is
// opened as ":memory:" (see NewMemory), so there is no file on disk to hand
// over, and JSON is what actually gets persisted. But JSON is CashFlux's shape.
// A .sqlite file is nobody's shape in particular — it opens in the sqlite3 CLI,
// in a GUI browser, in Python, in a spreadsheet importer — which makes it the
// export you want when the destination is not this app.
//
// The bytes come from SQLite's own backup machinery (ext/serdes drives
// sqlite3_backup into a slice-backed VFS), so the result is a real database
// image, byte-for-byte what SQLite would have written to disk. It is not a
// rendering of the data that happens to have a .sqlite extension.

// sqliteFileHeader is the 16-byte magic every SQLite database file starts with.
// Checked before anything else on import so a user who picked the wrong file
// gets "that is not a database" rather than a parse error from three layers
// down, or — worse — a half-applied import.
const sqliteFileHeader = "SQLite format 3\x00"

// ErrNotSQLiteFile is returned when the bytes offered for import are not a
// SQLite database at all.
var ErrNotSQLiteFile = errors.New("store: that file is not a SQLite database")

// ErrNotCashFluxDatabase is returned when the bytes ARE a SQLite database but
// not one of ours. A stranger's database opening cleanly and then presenting as
// an empty household is the failure worth preventing here: it looks like a
// successful import that wiped everything.
var ErrNotCashFluxDatabase = errors.New("store: that database was not written by CashFlux")

// serdesMu serializes calls into ext/serdes.
//
// That package hands the file to its VFS through a single package-level
// buffered channel, so two concurrent Serialize/Deserialize calls anywhere in
// the process can pick up each other's buffer. Nothing in the app does this
// concurrently today; the mutex is here so that staying true is not a property
// of call sites nobody is checking.
var serdesMu sync.Mutex

// SerializeSQLite returns the store as a standalone SQLite database file.
//
// The database is VACUUMed first. An in-memory SQLite database carries the free
// pages of everything ever deleted from it, and this one is rewritten wholesale
// on every Load — so without the vacuum the exported file is dominated by dead
// space that has nothing to do with how much the user actually has.
//
// PRAGMA user_version records the dataset schema version in the file itself, so
// an import can tell what it is reading instead of assuming it is current.
func (s *SQLiteStore) SerializeSQLite() ([]byte, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: not configured")
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return nil, fmt.Errorf("store: stamp schema version: %w", err)
	}
	if _, err := s.db.Exec("VACUUM"); err != nil {
		return nil, fmt.Errorf("store: vacuum before export: %w", err)
	}
	return serializeConn(s.db, "main")
}

// serializeConn pulls the raw SQLite handle out of a database/sql pool and asks
// SQLite to back the schema up into memory.
func serializeConn(db *sql.DB, schema string) ([]byte, error) {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("store: acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	var out []byte
	err = conn.Raw(func(driverConn any) error {
		raw, ok := driverConn.(sqlitedriver.Conn)
		if !ok {
			return fmt.Errorf("store: driver connection is %T, not a SQLite connection", driverConn)
		}
		serdesMu.Lock()
		defer serdesMu.Unlock()
		data, serErr := serdes.Serialize(raw.Raw(), schema)
		if serErr != nil {
			return serErr
		}
		out = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: serialize database: %w", err)
	}
	return out, nil
}

// LooksLikeSQLiteFile reports whether data begins with SQLite's file magic. It
// is a cheap pre-check, not a validation: a truncated or corrupt database has
// the same first sixteen bytes as a good one.
func LooksLikeSQLiteFile(data []byte) bool {
	return len(data) >= len(sqliteFileHeader) && string(data[:len(sqliteFileHeader)]) == sqliteFileHeader
}

// ImportSQLiteFile reads a SQLite database file and returns the dataset it
// holds, migrated to the current schema.
//
// It deliberately returns a Dataset rather than loading anything. The file is
// opened in a SCRATCH database and read through the ordinary Snapshot path, so
// a file that turns out to be corrupt, foreign, or newer than this build fails
// with the live store untouched. Deserializing straight into the live database
// would be faster and would leave a user whose file was wrong with no app to
// fix it from — the import would have already destroyed the schema it was
// checking.
func ImportSQLiteFile(data []byte) (Dataset, error) {
	if !LooksLikeSQLiteFile(data) {
		return Dataset{}, ErrNotSQLiteFile
	}

	scratch, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return Dataset{}, fmt.Errorf("store: open scratch database: %w", err)
	}
	// The in-memory database only exists for as long as its connection does.
	scratch.SetMaxOpenConns(1)
	defer func() { _ = scratch.Close() }()

	if err := deserializeInto(scratch, "main", data); err != nil {
		return Dataset{}, err
	}

	tables, err := tableNames(scratch)
	if err != nil {
		return Dataset{}, err
	}
	if missing := missingTables(tables); len(missing) > 0 {
		return Dataset{}, fmt.Errorf("%w (missing %s)", ErrNotCashFluxDatabase, strings.Join(missing, ", "))
	}

	version, err := userVersion(scratch)
	if err != nil {
		return Dataset{}, err
	}

	ds, err := (&SQLiteStore{db: scratch}).Snapshot()
	if err != nil {
		return Dataset{}, fmt.Errorf("store: read imported database: %w", err)
	}
	// Snapshot stamps the CURRENT schema version unconditionally, which is right
	// for the live store and wrong here: this dataset came out of a file that may
	// predate this build. Restore what the file actually claimed before migrating,
	// or an older file would skip every migration by asserting it was already
	// current. A file written before user_version was stamped reads as 0, which
	// migrate treats as the initial release.
	ds.SchemaVersion = version
	migrated, err := migrate(ds)
	if err != nil {
		return Dataset{}, err
	}
	return migrated, nil
}

// deserializeInto replaces schema's contents with the given database image.
func deserializeInto(db *sql.DB, schema string, data []byte) error {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("store: acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	err = conn.Raw(func(driverConn any) error {
		raw, ok := driverConn.(sqlitedriver.Conn)
		if !ok {
			return fmt.Errorf("store: driver connection is %T, not a SQLite connection", driverConn)
		}
		serdesMu.Lock()
		defer serdesMu.Unlock()
		return serdes.Deserialize(raw.Raw(), schema, data)
	})
	if err != nil {
		// A corrupt or truncated file fails here, and the message SQLite gives is
		// not one to show a person, so it is wrapped in the sentinel the caller
		// can turn into plain copy.
		return fmt.Errorf("%w: %v", ErrNotSQLiteFile, err)
	}
	return nil
}

// tableNames lists the tables in a database.
func tableNames(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_schema WHERE type = 'table'`)
	if err != nil {
		return nil, fmt.Errorf("store: list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("store: read table name: %w", err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list tables: %w", err)
	}
	return out, nil
}

// userVersion reads the schema version stamped into the file.
func userVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("store: read schema version: %w", err)
	}
	return version, nil
}

// missingTables returns the tables sqliteSchema creates that the given database
// does not have, sorted the way sqliteSchema declares them.
func missingTables(have map[string]bool) []string {
	var missing []string
	for _, name := range schemaTableNames() {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// schemaTableNames parses sqliteSchema for the tables it creates.
//
// Derived from the schema rather than kept as a second hand-written list, so a
// table added to one cannot be forgotten in the other — a drift that would show
// up as an import silently accepting a database missing whatever was added.
func schemaTableNames() []string {
	const prefix = "CREATE TABLE IF NOT EXISTS "
	var names []string
	for _, line := range strings.Split(sqliteSchema, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if idx := strings.IndexAny(rest, " ("); idx > 0 {
			names = append(names, rest[:idx])
		}
	}
	return names
}
