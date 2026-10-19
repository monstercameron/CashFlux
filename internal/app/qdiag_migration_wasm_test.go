// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func qdiagFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "qdiag", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func qdiagHydrate(t *testing.T, raw, head, generation string, records map[string]string) *appstate.App {
	t.Helper()
	a, err := appstate.New(os.Stderr, false)
	if err != nil {
		t.Fatal(err)
	}
	oldDefault := appstate.Default
	appstate.Default = a
	oldHydrated, oldLocal := hydrationSucceeded, hadLocalDataset
	oldQuarantined, oldPending := datasetQuarantined, quarantinePending
	oldRaw, oldAvailable := quarantineRaw, quarantineAvailable
	oldHApp, oldHCursor, oldHRev := hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision
	oldHosted := currentHostedHydration()
	oldDatasetState := hydratedDatasetState
	oldMyGen := datasetMyGen
	hydrationSucceeded, hadLocalDataset = false, false
	datasetQuarantined, quarantinePending = false, false
	quarantineRaw, quarantineAvailable = "", false
	hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = nil, 0, 0
	setHostedHydration(hostedHydrationStatus{})
	values := map[string]string{
		datasetStoreKey: raw,
		datasetHeadKey:  head,
		datasetGenKey:   generation,
	}
	for key, value := range records {
		values[key] = value
	}
	snapshot := durableDatasetSnapshot{
		SecureSnapshot: browserstore.SecureSnapshot{Values: values, Errors: map[string]error{}},
		workspace:      recordWorkspace(),
	}
	if err := hydrateDurableDataset(a, snapshot); err != nil {
		t.Fatalf("hydrate historical dataset: %v", err)
	}
	hydrationSucceeded, hadLocalDataset = true, true
	t.Cleanup(func() {
		hydrationSucceeded, hadLocalDataset = oldHydrated, oldLocal
		datasetQuarantined, quarantinePending = oldQuarantined, oldPending
		quarantineRaw, quarantineAvailable = oldRaw, oldAvailable
		hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = oldHApp, oldHCursor, oldHRev
		hydratedDatasetState = oldDatasetState
		datasetMyGen = oldMyGen
		setHostedHydration(oldHosted)
		appstate.Default = oldDefault
		_ = a.Store().Close()
	})
	return a
}

func TestQDIAGHistoricalDatasetFixturesHydrate(t *testing.T) {
	for _, name := range []string{
		"pre_solp2_plain.json",
		"pre_liability_sign.json",
		"pre_transfer_group.json",
		"pre_rule_migration.json",
	} {
		t.Run(name, func(t *testing.T) {
			a := qdiagHydrate(t, qdiagFixture(t, name), "", "fixture-generation", nil)
			if !hydrationSucceeded || datasetQuarantined || quarantinePending {
				t.Fatalf("valid historical dataset was quarantined: success=%v quarantined=%v pending=%v storage=%v", hydrationSucceeded, datasetQuarantined, quarantinePending, browserstore.InitError())
			}
			switch name {
			case "pre_solp2_plain.json":
				if len(a.Transactions()) != 1 {
					t.Fatalf("plain dataset rows = %d, want 1", len(a.Transactions()))
				}
			case "pre_liability_sign.json":
				if got := a.Accounts()[0].LiabilitySign; got != domain.LiabilityNegativeOwed {
					t.Fatalf("legacy liability sign = %q", got)
				}
			case "pre_transfer_group.json":
				rows := a.Transactions()
				if len(rows) != 2 || rows[0].TransferGroupID == "" || rows[0].TransferGroupID != rows[1].TransferGroupID {
					t.Fatalf("legacy transfer pair not migrated: %+v", rows)
				}
			case "pre_rule_migration.json":
				data, err := a.ExportJSONRedacted()
				if err != nil {
					t.Fatal(err)
				}
				ds, err := store.Import(data)
				if err != nil {
					t.Fatal(err)
				}
				if len(ds.Rules) != 2 || ds.SettingsKV["migration.rules.signed-amount.v1"] != "1" {
					t.Fatalf("legacy rules not migrated: %+v", ds)
				}
			}
		})
	}
}

func TestQDIAGSOLP2PreviousSchemaCheckpointAndJournalHydrate(t *testing.T) {
	checkpoint := qdiagFixture(t, "pre_solp2_checkpoint.json")
	plain, recordFormat, err := checkpointDatasetJSON([]byte(checkpoint))
	if err != nil || !recordFormat {
		t.Fatalf("read SOLP2 fixture checkpoint: recordFormat=%v err=%v", recordFormat, err)
	}
	ds, err := store.Import(plain)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Load(ds); err != nil {
		t.Fatal(err)
	}
	from, err := st.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.GetTransaction("t1")
	if err != nil || !ok {
		t.Fatalf("fixture transaction missing: found=%v err=%v", ok, err)
	}
	row.Note = "saved by SOLP2 journal"
	if err := st.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	to, err := st.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	page, err := st.CaptureChanges(from, to, 64)
	if err != nil || len(page.Changes) != 1 {
		t.Fatalf("capture historical journal record: changes=%d err=%v", len(page.Changes), err)
	}
	workspace, generation := recordWorkspace(), "historical-generation"
	prefix := recordWorkspacePrefix(workspace) + "historical:"
	recordKey, batchKey := prefix+"0", prefix+"batch"
	recordJSON, err := json.Marshal(page.Changes[0])
	if err != nil {
		t.Fatal(err)
	}
	nodeJSON, err := json.Marshal(datasetRecordNode{Keys: []string{recordKey}})
	if err != nil {
		t.Fatal(err)
	}
	headJSON, err := json.Marshal(datasetRecordHead{Version: 1, Workspace: workspace, BaseHash: checkpointHash(checkpoint), Generation: generation, Tail: batchKey})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{recordKey: string(recordJSON), batchKey: string(nodeJSON)}
	a := qdiagHydrate(t, checkpoint, string(headJSON), generation, keys)
	if !hydrationSucceeded || datasetQuarantined || quarantinePending {
		t.Fatalf("historical checkpoint/journal quarantined: success=%v quarantined=%v pending=%v storage=%v", hydrationSucceeded, datasetQuarantined, quarantinePending, browserstore.InitError())
	}
	loaded, ok := a.TransactionByID("t1")
	if !ok || loaded.Note != "saved by SOLP2 journal" {
		t.Fatalf("journal edit missing after hydration: row=%+v found=%v", loaded, ok)
	}
}

func TestQDIAGHydrateProfile3300(t *testing.T) {
	ds := store.SampleDataset()
	ds.Transactions = make([]domain.Transaction, 3300)
	for i := range ds.Transactions {
		ds.Transactions[i] = domain.Transaction{
			ID: fmt.Sprintf("qdiag-%04d", i), AccountID: ds.Accounts[0].ID,
			CategoryID: ds.Categories[0].ID, Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
			Desc: "Historical grocery transaction", Amount: money.New(-4200, "USD"),
		}
	}
	raw, err := store.Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	a := qdiagHydrate(t, string(raw), "", "profile-generation", nil)
	duration := time.Since(started)
	t.Logf("QDIAG boot hydrate: %d transactions, %d dataset bytes, %.3f ms", len(a.Transactions()), len(raw), float64(duration.Microseconds())/1000)
	if !hydrationSucceeded || datasetQuarantined || quarantinePending || len(a.Transactions()) != 3300 {
		t.Fatalf("profile dataset failed hydration: success=%v quarantined=%v pending=%v rows=%d storage=%v", hydrationSucceeded, datasetQuarantined, quarantinePending, len(a.Transactions()), browserstore.InitError())
	}
}

func TestQDIAGCheckpointAcceptsOlderWriterSchemaCeiling(t *testing.T) {
	dataset := []byte(qdiagFixture(t, "pre_solp2_plain.json"))
	oldCheckpoint := []byte(qdiagFixture(t, "pre_solp2_checkpoint.json"))
	got, recordFormat, err := checkpointDatasetJSON(oldCheckpoint)
	if err != nil || !recordFormat || !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(dataset)) {
		t.Fatalf("prior schema ceiling rejected: format=%v err=%v", recordFormat, err)
	}
	future := []byte(fmt.Sprintf(`{"schemaVersion":%d,"cashfluxRecordCheckpoint":1,"dataset":%s}`, store.SchemaVersion+2, dataset))
	if _, _, err := checkpointDatasetJSON(future); err == nil {
		t.Fatal("future checkpoint unexpectedly accepted")
	}
	if _, _, err := checkpointDatasetJSON([]byte(`{"schemaVersion":2,"cashfluxRecordCheckpoint":99,"dataset":{}}`)); err == nil {
		t.Fatal("corrupt/unknown journal format unexpectedly accepted")
	}
}
