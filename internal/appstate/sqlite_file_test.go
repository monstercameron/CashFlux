// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	sqlitedriver "github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/serdes"
)

// The app-level half of the SQLite file export: everything the store layer
// cannot see on its own — artifact bytes living in the blob store, the live
// database being left alone, and a bad file changing nothing.

func appWithHousehold(t *testing.T) *App {
	t.Helper()
	a, err := New(&bytes.Buffer{}, true) // seeded sample household
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

// The plain promise: export the household, import it into a fresh app, and it
// is the same household.
func TestExportImportSQLiteRoundTrip(t *testing.T) {
	source := appWithHousehold(t)
	before, err := source.ExportJSON()
	if err != nil {
		t.Fatalf("baseline export: %v", err)
	}

	data, err := source.ExportSQLite()
	if err != nil {
		t.Fatalf("ExportSQLite: %v", err)
	}
	if !store.LooksLikeSQLiteFile(data) {
		t.Fatal("ExportSQLite did not produce a SQLite database file")
	}

	target, err := New(&bytes.Buffer{}, false) // empty app
	if err != nil {
		t.Fatalf("New empty: %v", err)
	}
	if len(target.Transactions()) != 0 {
		t.Fatal("the target app was expected to start empty")
	}
	if err := target.ImportSQLite(data); err != nil {
		t.Fatalf("ImportSQLite: %v", err)
	}

	after, err := target.ExportJSON()
	if err != nil {
		t.Fatalf("export after import: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("the household changed on its way through a SQLite file")
	}
	if len(target.Transactions()) == 0 {
		t.Fatal("no transactions arrived")
	}
}

// Artifact image bytes live in IndexedDB, not in the dataset. The exported file
// has to carry them anyway, or a backup restored on a new device silently loses
// every receipt.
func TestExportSQLiteCarriesArtifactBytes(t *testing.T) {
	source, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blobs := newMemBlobStore()
	source.SetBlobStore(blobs)

	imageBytes := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0xff, 0x00, 0x42}
	// StoreBlobForArtifact is the real ingest path: it moves Bytes into the blob
	// store and clears them from the record, which is the state this export has
	// to reconstitute from.
	stored, err := source.StoreBlobForArtifact(domain.Artifact{
		ID: "art1", Kind: artifacts.KindImage, MIME: "image/png",
		Name: "receipt.png", Bytes: imageBytes, Size: len(imageBytes), CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("StoreBlobForArtifact: %v", err)
	}
	if err := source.PutArtifact(stored); err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}

	// Precondition: the bytes are in the blob store, NOT in the dataset.
	live, err := source.ExportJSON()
	if err != nil {
		t.Fatalf("live export: %v", err)
	}
	if bytes.Contains(live, []byte("iVBORw")) {
		t.Fatal("precondition failed: artifact bytes are already inline in the dataset")
	}

	data, err := source.ExportSQLite()
	if err != nil {
		t.Fatalf("ExportSQLite: %v", err)
	}

	target, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New target: %v", err)
	}
	targetBlobs := newMemBlobStore()
	target.SetBlobStore(targetBlobs)
	if err := target.ImportSQLite(data); err != nil {
		t.Fatalf("ImportSQLite: %v", err)
	}

	_, got, ok, err := targetBlobs.Get("art1")
	if err != nil || !ok {
		t.Fatalf("artifact bytes did not reach the target blob store: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, imageBytes) {
		t.Fatalf("artifact bytes changed in transit: got %v, want %v", got, imageBytes)
	}
	// And they were stripped from the record on the way in, so the next autosave
	// does not push them back into localStorage.
	arts := target.Artifacts()
	if len(arts) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(arts))
	}
	if len(arts[0].Bytes) != 0 {
		t.Fatalf("artifact record still carries %d inline bytes", len(arts[0].Bytes))
	}
}

// Exporting must not disturb the live database — in particular it must not
// write the rehydrated artifact bytes back into it, which would undo the whole
// reason they live in IndexedDB.
func TestExportSQLiteLeavesTheLiveStoreAlone(t *testing.T) {
	app, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	app.SetBlobStore(newMemBlobStore())
	stored, err := app.StoreBlobForArtifact(domain.Artifact{
		ID: "art1", Kind: artifacts.KindImage, MIME: "image/png",
		Name: "receipt.png", Bytes: []byte{1, 2, 3, 4, 5}, Size: 5, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("StoreBlobForArtifact: %v", err)
	}
	if err := app.PutArtifact(stored); err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}

	before, err := app.ExportJSON()
	if err != nil {
		t.Fatalf("before: %v", err)
	}
	if _, err = app.ExportSQLite(); err != nil {
		t.Fatalf("ExportSQLite: %v", err)
	}
	after, err := app.ExportJSON()
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("exporting a SQLite file modified the live database")
	}
	if arts := app.Artifacts(); len(arts) != 1 || len(arts[0].Bytes) != 0 {
		t.Fatal("the export wrote artifact bytes back into the live record")
	}
}

// A rejected file must leave the household exactly as it was. This is the whole
// reason the import reads into a scratch database first.
func TestImportSQLiteRejectsBadFilesWithoutTouchingData(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"a JSON export, not a database", []byte(`{"members":[]}`), store.ErrNotSQLiteFile},
		{"empty file", nil, store.ErrNotSQLiteFile},
		{"a database from another program", foreignDatabase(), store.ErrNotCashFluxDatabase},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := appWithHousehold(t)
			before, err := app.ExportJSON()
			if err != nil {
				t.Fatalf("baseline: %v", err)
			}
			txnsBefore := len(app.Transactions())
			if txnsBefore == 0 {
				t.Fatal("precondition: the app should start with data to lose")
			}

			if err := app.ImportSQLite(tc.data); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}

			after, err := app.ExportJSON()
			if err != nil {
				t.Fatalf("after: %v", err)
			}
			if string(before) != string(after) {
				t.Fatal("a refused import still changed the household")
			}
			if got := len(app.Transactions()); got != txnsBefore {
				t.Fatalf("transactions = %d, want %d — a refused import lost data", got, txnsBefore)
			}
		})
	}
}

// foreignDatabase builds a real SQLite database file that is not one of ours —
// the "opened fine, but it is somebody else's data" case.
func foreignDatabase() []byte {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)`); err != nil {
		return nil
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil
	}
	defer func() { _ = conn.Close() }()
	var out []byte
	_ = conn.Raw(func(dc any) error {
		raw, ok := dc.(sqlitedriver.Conn)
		if !ok {
			return nil
		}
		data, serErr := serdes.Serialize(raw.Raw(), "main")
		out = data
		return serErr
	})
	return out
}

// An empty app exports and imports cleanly — the first thing a new user could
// do, and the case where an "is there anything to export" shortcut would bite.
func TestExportImportSQLiteOfAnEmptyApp(t *testing.T) {
	source, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data, err := source.ExportSQLite()
	if err != nil {
		t.Fatalf("ExportSQLite: %v", err)
	}
	target, err := New(&bytes.Buffer{}, true) // starts WITH the sample
	if err != nil {
		t.Fatalf("New target: %v", err)
	}
	if err := target.ImportSQLite(data); err != nil {
		t.Fatalf("ImportSQLite: %v", err)
	}
	if got := len(target.Transactions()); got != 0 {
		t.Fatalf("transactions after importing an empty household = %d, want 0", got)
	}
}

// A household that has been edited since its last export round-trips too — the
// export reads the live state, not a stale snapshot.
func TestExportSQLiteReflectsRecentEdits(t *testing.T) {
	app := appWithHousehold(t)
	accounts := app.Accounts()
	if len(accounts) == 0 {
		t.Fatal("sample household has no accounts")
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "just-added", AccountID: accounts[0].ID, Desc: "Added after load",
		Date: time.Now(), Amount: money.New(-777, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	data, err := app.ExportSQLite()
	if err != nil {
		t.Fatalf("ExportSQLite: %v", err)
	}
	target, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := target.ImportSQLite(data); err != nil {
		t.Fatalf("ImportSQLite: %v", err)
	}
	found := false
	for _, txn := range target.Transactions() {
		if txn.ID == "just-added" && txn.Desc == "Added after load" {
			found = true
		}
	}
	if !found {
		t.Fatal("an edit made after load did not reach the exported file")
	}
}
