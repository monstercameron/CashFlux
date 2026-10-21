// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestJRNLUnchangedCheckpointRetainsValidatedParse(t *testing.T) {
	raw, err := store.Export(jrnlDataset())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := encodeRecordCheckpointGeneration(raw, "parsed-root")
	for _, backup := range []bool{false, true} {
		values := map[string]string{datasetStoreKey: checkpoint, datasetGenKey: "parsed-root"}
		if backup {
			values[datasetStoreKey] = "damaged live checkpoint"
			values[datasetCheckpointBackupKey] = encodeCheckpointBackup("parsed-workspace", checkpoint, "")
		}
		s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: values, Errors: map[string]error{}}, "parsed-workspace"}
		portable, parsed, repair, err := recoverDatasetSnapshotParsed(s)
		if err != nil || parsed == nil || repair != backup {
			t.Fatalf("backup=%v parsed=%v repair=%v err=%v", backup, parsed != nil, repair, err)
		}
		if portable != string(raw) {
			t.Fatal("unchanged checkpoint bytes differ")
		}
		a := jrnlApp(t)
		if err := a.ImportParsedDataset(parsed); err != nil {
			t.Fatal(err)
		}
		if len(a.Transactions()) != len(jrnlDataset().Transactions) {
			t.Fatal("parsed import lost checkpoint rows")
		}
	}
}

func TestJRNLReplayedCheckpointDiscardsBaseParse(t *testing.T) {
	f := jrnlFixture(t)
	s := jrnlRead(t)
	portable, parsed, _, err := recoverDatasetSnapshotParsed(s)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != nil {
		t.Fatal("journal replay retained the stale base parse")
	}
	a := jrnlApp(t)
	if err := a.ImportJSON([]byte(portable)); err != nil {
		t.Fatal(err)
	}
	jrnlAssertRow(t, a, f.row.ID, f.row.Desc)
}
