// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// A separate workspace keeps earlier SOLP fixtures out of recovery snapshots.
const jrnlFixtureWorkspace = "jrnl-workspace"

func jrnlFixture(t *testing.T) solp22Fixture {
	return jrnlFixtureForWorkspace(t, jrnlFixtureWorkspace)
}

func jrnlFixtureForWorkspace(t *testing.T, workspaceID string) solp22Fixture {
	t.Helper()
	solp22Storage(t)
	oldHydrated := hydratedDatasetState
	oldNotice := durableNoticeApp
	oldBlocked, oldReady, oldRecovery := staleDatasetBlocked, staleRecoveryReady, staleDatasetRecoveryKey
	previous := map[string]string{}
	initial := map[string]bool{}
	for _, key := range browserstore.Keys() {
		initial[key] = true
	}
	for _, key := range []string{datasetCheckpointBackupKey, datasetRecoverySourceKey, seededFlagKey} {
		previous[key] = browserstore.GetString(key)
	}
	cleared := map[string]string{}
	for key := range initial {
		if strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
			if value := browserstore.GetString(key); value != "" {
				previous[key] = value
				cleared[key] = ""
			}
		}
	}
	if len(cleared) > 0 {
		solp22Batch(t, cleared, nil)
	}
	t.Cleanup(func() {
		browserstore.Barrier()
		solp22Batch(t, previous, nil)
		var detachedRecords []string
		for _, key := range browserstore.Keys() {
			if !initial[key] && strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
				detachedRecords = append(detachedRecords, key)
			}
			if !initial[key] && (strings.HasPrefix(key, journalRecoveryPrefix) || strings.HasPrefix(key, quarantinePrefix)) {
				if err := enc31Wait(t, func(done func(error)) { browserstore.RemoveSecure(key, done) }); err != nil {
					t.Error(err)
				}
			}
		}
		jrnlRemoveFixtureRecords(t, detachedRecords)
		hydratedDatasetState, durableNoticeApp = oldHydrated, oldNotice
		staleDatasetBlocked, staleRecoveryReady, staleDatasetRecoveryKey = oldBlocked, oldReady, oldRecovery
		uistate.ClearPersistFailure("dataset")
	})
	return solp22JournalDataset(t, jrnlDataset(), workspaceID)
}

// Recovery tests need a household and real journal rows, without unrelated demo
// plans, documents, holdings and five years of balance snapshots in every copy.
// Keep the same accounts, categories, members and row images as the original
// fixture; the production encryption, replay and 128-key threshold are unchanged.
func jrnlDataset() store.Dataset {
	sample := store.SampleDataset()
	return store.Dataset{
		Members: sample.Members, Accounts: sample.Accounts, Categories: sample.Categories,
		Transactions: sample.Transactions[:2], Settings: sample.Settings,
	}
}

// Remove detached synthetic padding in one transaction, after restoring the
// original metadata. Leaving encrypted tombstones here makes every later fixture
// decrypt another 128 abandoned keys. This helper refuses real browser storage.
func jrnlRemoveFixtureRecords(t *testing.T, keys []string) {
	t.Helper()
	if len(keys) == 0 {
		return
	}
	if !js.Global().Get("process").Truthy() || !js.Global().Get("__fakeIDB").Truthy() {
		t.Fatal("JRNL cleanup requires the isolated Node IndexedDB shim")
	}
	finished := make(chan string, 1)
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any { finished <- args[0].String(); return nil })
	defer callback.Release()
	items := make([]any, len(keys))
	for i, key := range keys {
		items[i] = key
	}
	js.Global().Get("Function").New("keys", "done", `
const request = indexedDB.open("cashflux-kv", 1);
request.onerror = () => done("could not open the isolated fixture database");
request.onsuccess = () => {
  const database = request.result;
  const transaction = database.transaction("kv", "readwrite", {durability: "strict"});
  transaction.oncomplete = () => { database.close(); done(""); };
  transaction.onabort = () => { database.close(); done("fixture cleanup transaction aborted"); };
  for (const key of keys) transaction.objectStore("kv").delete(key);
};`).Invoke(js.ValueOf(items), callback)
	select {
	case err := <-finished:
		if err != "" {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("fixture cleanup did not finish")
	}
	for _, key := range keys {
		// Remove also evicts the cache entry. An empty SetMemory value would
		// leave Keys() advertising detached rows to later financial wipes.
		browserstore.Remove(key)
	}
	browserstore.Barrier()
	for _, key := range keys {
		if _, present := browserstore.Get(key); present {
			t.Fatalf("detached fixture record remains cached: %s", key)
		}
	}
}

func jrnlRead(t *testing.T) durableDatasetSnapshot {
	t.Helper()
	s, err := readDurableDatasetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func jrnlApp(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.SetDurableWriter(nil); a.Store().Close() })
	return a
}

func jrnlAssertRow(t *testing.T, a *appstate.App, id, desc string) {
	t.Helper()
	row, have, err := a.Store().GetTransaction(id)
	if err != nil || !have || row.Desc != desc {
		t.Fatalf("user row lost: %+v %v", row, err)
	}
}

func TestJRNLGenerationSplitRecoversUserDataInsteadOfQuarantine(t *testing.T) {
	f := jrnlFixture(t)
	s := jrnlRead(t)
	old := s.Values[datasetGenKey]
	solp22Batch(t, map[string]string{datasetGenKey: "interrupted-legacy-stamp"}, nil)
	s = jrnlRead(t)
	_, err := readDatasetRecords(s.Values[datasetStoreKey], s.Values[datasetHeadKey], s.Values[datasetGenKey], s.workspace, s.Read)
	if err == nil || err.Error() != "the saved checkpoint and change journal have inconsistent generations" {
		t.Fatalf("incident not reproduced: %v", err)
	}
	loaded := jrnlApp(t)
	appstate.Default = loaded
	// Reproduce a reload's stale cache too; hydration must use durable metadata.
	browserstore.SetMemory(datasetGenKey, old)
	hydrateDataset()
	if !hydrationSucceeded || datasetQuarantined || quarantinePending {
		t.Fatal("authenticated household was quarantined")
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
	if hydratedDatasetState.snapshot.Values[datasetGenKey] == "interrupted-legacy-stamp" {
		t.Fatal("split was not repaired")
	}
	var kept bool
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, journalRecoveryPrefix) {
			raw, _, err := browserstore.GetSecure(key)
			var copy recordRecovery
			if err != nil || json.Unmarshal([]byte(raw), &copy) != nil {
				t.Fatal("recovery copy unreadable")
			}
			if copy.Checkpoint == f.checkpoint && copy.Head == s.Values[datasetHeadKey] && len(copy.Records) > 0 {
				kept = true
			}
		}
	}
	if !kept || len(quarantinedDownloadBytes()) == 0 {
		t.Fatal("original checkpoint and journal not available to download")
	}
	// A second boot must not repair the already repaired journal again.
	before := readDatasetGen()
	reloaded := jrnlApp(t)
	appstate.Default = reloaded
	hydrateDataset()
	jrnlAssertRow(t, reloaded, f.row.ID, f.row.Desc)
	if readDatasetGen() != before {
		t.Fatal("recovery repeats every boot")
	}
	newDatasetAutosave(reloaded, browserstore.SetSecureBatch, nil)
	row := reloaded.Transactions()[0]
	row.Desc = "edited after recovered reload"
	if err := reloaded.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) { reloaded.RequestDurable(func(r appstate.DurableResult) { done(r.Err) }) }); err != nil {
		t.Fatalf("recovered writer adopted stale cache: %v", err)
	}
}

func TestJRNLTwoAppsReloadAndCompactionRace(t *testing.T) {
	for _, winner := range []string{"append", "compaction"} {
		t.Run(winner, func(t *testing.T) {
			f := jrnlFixture(t)
			initial := jrnlRead(t)
			a, b := jrnlApp(t), jrnlApp(t)
			commits := make(chan solp2Commit, 4)
			writer := func(app *appstate.App) (func(), func()) {
				appstate.Default = app
				if err := hydrateDurableDataset(app, initial); err != nil {
					t.Fatal(err)
				}
				return newDatasetAutosave(app, func(values, expected map[string]string, done func(error)) {
					commits <- solp2Commit{values, expected, done}
				}, nil)
			}
			_, _ = writer(b)
			// Both have loaded exactly the same committed household, like two tabs.
			appstate.Default = b
			row := b.Transactions()[0]
			row.Desc = "second tab append"
			if err := b.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			bResult := make(chan appstate.DurableResult, 1)
			b.RequestDurable(func(r appstate.DurableResult) { bResult <- r })
			appendCommit := waitSOLP2Commit(t, commits)
			if _, full := appendCommit.values[datasetStoreKey]; full {
				t.Fatal("append unexpectedly checkpointed")
			}
			// The other tab reaches TC4N9's actual 128-key threshold while B's
			// append is prepared. Repeated authenticated row images keep the
			// fixture small without bypassing production threshold/compaction.
			var head datasetRecordHead
			_ = json.Unmarshal([]byte(initial.Values[datasetHeadKey]), &head)
			var node datasetRecordNode
			_ = json.Unmarshal([]byte(initial.Values[head.Tail]), &node)
			padding := map[string]string{}
			for len(node.Keys) < journalCheckpointRecordLimit-1 {
				key := fmt.Sprintf("%s%s:threshold-padding:%d", recordWorkspacePrefix(initial.workspace), head.Generation, len(node.Keys))
				padding[key] = initial.Values[node.Keys[0]]
				node.Keys = append(node.Keys, key)
			}
			raw, _ := json.Marshal(node)
			padding[head.Tail] = string(raw)
			solp22Batch(t, padding, nil)
			initial = jrnlRead(t)
			_, _ = writer(a)
			appstate.Default = a
			rowA := a.Transactions()[0]
			rowA.Desc = "first tab compacted"
			if err := a.PutTransaction(rowA); err != nil {
				t.Fatal(err)
			}
			aResult := make(chan appstate.DurableResult, 1)
			a.RequestDurable(func(r appstate.DurableResult) { aResult <- r })
			compactCommit := waitSOLP2Commit(t, commits)
			if compactCommit.values[datasetStoreKey] == "" || compactCommit.values[datasetHeadKey] != "" {
				t.Fatal("no compaction commit")
			}
			// A reload between preparation and the strict commit sees only the old
			// complete state; neither outstanding operation is acknowledged.
			mid := jrnlRead(t)
			portable, _, err := recoverDatasetSnapshot(mid)
			if err != nil {
				t.Fatal(err)
			}
			solp22AssertRow(t, portable, f.row)
			select {
			case <-bResult:
				t.Fatal("early durability acknowledgement")
			default:
			}
			first, second := appendCommit, compactCommit
			winningApp, losingApp := b, a
			want := row.Desc
			if winner == "compaction" {
				first, second = compactCommit, appendCommit
				winningApp, losingApp = a, b
				want = rowA.Desc
			}
			appstate.Default = winningApp
			if err := enc31Wait(t, func(done func(error)) {
				browserstore.SetSecureBatch(first.values, first.expected, func(err error) { first.done(err); done(err) })
			}); err != nil {
				t.Fatal(err)
			}
			if winner == "append" {
				durableHead, _, err := browserstore.GetSecure(datasetHeadKey)
				if err != nil {
					t.Fatal(err)
				}
				_, err = readDatasetRecords(initial.Values[datasetStoreKey], durableHead, initial.Values[datasetGenKey], initial.workspace, browserstore.GetSecure)
				if err == nil || err.Error() != "the saved checkpoint and change journal have inconsistent generations" {
					t.Fatalf("two-App reload mixture did not reproduce the incident: %v", err)
				}
			}
			if _, _, err := materializeLocalDataset(); err != nil {
				t.Fatalf("full-replacement boundary read a split snapshot: %v", err)
			}
			if winner == "append" {
				durableHead, _, err := browserstore.GetSecure(datasetHeadKey)
				if err != nil {
					t.Fatal(err)
				}
				_, err = readDatasetRecords(initial.Values[datasetStoreKey], durableHead, initial.Values[datasetGenKey], initial.workspace, browserstore.GetSecure)
				if err == nil || err.Error() != "the saved checkpoint and change journal have inconsistent generations" {
					t.Fatalf("two-App reload mixture did not reproduce the incident: %v", err)
				}
			}
			if _, _, err := materializeLocalDataset(); err != nil {
				t.Fatalf("full-replacement boundary read a split snapshot: %v", err)
			}
			appstate.Default = losingApp
			err = enc31Wait(t, func(done func(error)) {
				browserstore.SetSecureBatch(second.values, second.expected, func(err error) { second.done(err); done(err) })
			})
			if err == nil {
				t.Fatal("stale writer overwrote the winning dataset")
			}
			if !staleDatasetBlocked {
				t.Fatal("CAS loser did not enter TC4N9 stale-tab path")
			}
			if winner == "compaction" {
				if r := <-bResult; !errors.Is(r.Err, appstate.ErrStaleDataset) {
					t.Fatalf("CAS loser = %v", r.Err)
				}
			} else if r := <-aResult; !errors.Is(r.Err, appstate.ErrStaleDataset) {
				t.Fatalf("compaction CAS loser = %v", r.Err)
			}
			browserstore.Barrier()
			latest := jrnlRead(t)
			portable, _, err = recoverDatasetSnapshot(latest)
			if err != nil {
				t.Fatal(err)
			}
			ds, err := store.Import([]byte(portable))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, got := range ds.Transactions {
				if got.ID == row.ID && got.Desc == want {
					found = true
				}
			}
			if !found {
				t.Fatal("reload lost the winning committed edit")
			}
			resetStaleDatasetState()
			suspendAutosave = false
		})
	}
}

func TestJRNLRecoveryRetainsDivergentAndDamagedJournal(t *testing.T) {
	f := jrnlFixture(t)
	initial := jrnlRead(t)
	for _, scenario := range []string{"valid", "generation split", "foreign head", "missing row", "missing head", "cursor mismatch", "divergent branch"} {
		t.Run(scenario, func(t *testing.T) {
			s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: map[string]string{}, Errors: map[string]error{}}, initial.workspace}
			for key, value := range initial.Values {
				s.Values[key] = value
			}
			var head datasetRecordHead
			_ = json.Unmarshal([]byte(s.Values[datasetHeadKey]), &head)
			var node datasetRecordNode
			_ = json.Unmarshal([]byte(s.Values[head.Tail]), &node)
			wantEdit := true
			switch scenario {
			case "generation split":
				s.Values[datasetGenKey] = "other"
			case "foreign head":
				head.Workspace = "other-household"
			case "missing row":
				delete(s.Values, node.Keys[0])
				wantEdit = false
			case "missing head":
				s.Values[datasetHeadKey] = ""
			case "cursor mismatch":
				node.Through += 10
				raw, _ := json.Marshal(node)
				s.Values[head.Tail] = string(raw)
				wantEdit = false
			case "divergent branch":
				s.Values[recordWorkspacePrefix(s.workspace)+"orphan:0"] = `{"table":"transactions","id":"unmerged"}`
			}
			if scenario != "missing head" {
				raw, _ := json.Marshal(head)
				s.Values[datasetHeadKey] = string(raw)
			}
			portable, repair, err := recoverDatasetSnapshot(s)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "valid" && repair {
				t.Fatal("valid journal needed repair")
			}
			if scenario != "valid" && !repair {
				t.Fatal("incident not retained for recovery")
			}
			dataset, err := store.Import([]byte(portable))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range dataset.Transactions {
				if row.ID == f.row.ID && row.Desc == f.row.Desc {
					found = true
				}
			}
			if found != wantEdit {
				t.Fatalf("replay unsafe or lost recoverable row: edit=%v want=%v", found, wantEdit)
			}
			var copy recordRecovery
			_ = json.Unmarshal([]byte(snapshotRecoveryCopy(s)), &copy)
			if copy.Checkpoint != f.checkpoint {
				t.Fatal("original checkpoint not retained")
			}
			if scenario == "divergent branch" && copy.Records[recordWorkspacePrefix(s.workspace)+"orphan:0"] == "" {
				t.Fatal("authenticated divergent row silently dropped")
			}
		})
	}
}

func TestJRNLCheckpointFallbackAndGenuineCorruption(t *testing.T) {
	for _, backup := range []string{"same workspace", "wrong workspace", "replaced generation", "none"} {
		t.Run(backup, func(t *testing.T) {
			f := jrnlFixture(t)
			values := map[string]string{datasetStoreKey: "{damaged-checkpoint", datasetCheckpointBackupKey: ""}
			if backup != "none" {
				scope := recordWorkspace()
				if backup == "wrong workspace" {
					scope = "other"
				}
				values[datasetCheckpointBackupKey] = encodeCheckpointBackup(scope, f.checkpoint)
			}
			if backup == "replaced generation" {
				values[datasetHeadKey] = ""
				values[datasetGenKey] = "new-full-replacement"
			}
			solp22Batch(t, values, nil)
			loaded := jrnlApp(t)
			appstate.Default = loaded
			hydrateDataset()
			browserstore.Barrier()
			if backup == "same workspace" || backup == "replaced generation" {
				if !hydrationSucceeded || datasetQuarantined {
					t.Fatal("readable backup quarantined")
				}
				jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
			} else if hydrationSucceeded || !datasetQuarantined {
				t.Fatal("genuine corruption not quarantined")
			}
		})
	}
}

func TestJRNLRecoveryReplaysOnlyCausalPrefixAndKeepsForks(t *testing.T) {
	f := jrnlFixture(t)
	initial := jrnlRead(t)
	for _, scenario := range []string{"complete chain", "legacy tail after modern node", "missing last record", "missing tail batch", "wrong parent generation", "wrong checkpoint generation", "fork without head"} {
		t.Run(scenario, func(t *testing.T) {
			s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: map[string]string{}, Errors: map[string]error{}}, initial.workspace}
			for key, value := range initial.Values {
				s.Values[key] = value
			}
			var head datasetRecordHead
			_ = json.Unmarshal([]byte(s.Values[datasetHeadKey]), &head)
			row := f.row
			row.Desc = "second causal edit"
			after, _ := json.Marshal(row)
			change, _ := json.Marshal(store.RecordChange{Table: "transactions", ID: row.ID, After: after})
			prefix := recordWorkspacePrefix(s.workspace) + "second:"
			node := datasetRecordNode{Parent: head.Tail, Keys: []string{prefix + "0"}, Generation: "second", ParentGeneration: head.Generation, BaseHash: head.BaseHash, Workspace: s.workspace, From: head.Cursor, Through: head.Cursor + 1}
			if scenario == "wrong parent generation" {
				node.ParentGeneration = "unrelated"
			}
			raw, _ := json.Marshal(node)
			s.Values[prefix+"batch"], s.Values[prefix+"0"] = string(raw), string(change)
			head.Tail, head.Generation, head.Cursor = prefix+"batch", node.Generation, node.Through
			raw, _ = json.Marshal(head)
			s.Values[datasetHeadKey], s.Values[datasetGenKey] = string(raw), head.Generation
			want := f.row.Desc
			switch scenario {
			case "complete chain":
				want = row.Desc
			case "legacy tail after modern node":
				node = datasetRecordNode{Parent: node.Parent, Keys: node.Keys}
				encoded, _ := json.Marshal(node)
				s.Values[prefix+"batch"] = string(encoded)
				head.Cursor = 0
				encoded, _ = json.Marshal(head)
				s.Values[datasetHeadKey] = string(encoded)
				want = row.Desc
			case "missing last record":
				delete(s.Values, prefix+"0")
			case "missing tail batch":
				delete(s.Values, prefix+"batch")
			case "wrong checkpoint generation":
				data, _, _ := checkpointDatasetJSON([]byte(f.checkpoint))
				s.Values[datasetStoreKey] = encodeRecordCheckpointGeneration(data, "unrelated-base")
				// Rebind the hashes to isolate the root-generation check.
				base := checkpointHash(s.Values[datasetStoreKey])
				head.BaseHash = base
				for key, value := range s.Values {
					if strings.HasSuffix(key, ":batch") {
						var n datasetRecordNode
						if json.Unmarshal([]byte(value), &n) == nil {
							n.BaseHash = base
							encoded, _ := json.Marshal(n)
							s.Values[key] = string(encoded)
						}
					}
				}
				raw, _ = json.Marshal(head)
				s.Values[datasetHeadKey] = string(raw)
				baseDataset, _ := store.Import(data)
				for _, r := range baseDataset.Transactions {
					if r.ID == f.row.ID {
						want = r.Desc
					}
				}
			case "fork without head":
				fork := node
				fork.Generation = "fork"
				fork.Keys = []string{recordWorkspacePrefix(s.workspace) + "fork:0"}
				raw, _ := json.Marshal(fork)
				s.Values[recordWorkspacePrefix(s.workspace)+"fork:batch"] = string(raw)
				s.Values[fork.Keys[0]] = string(change)
				s.Values[datasetHeadKey] = ""
			}
			portable, repair, err := recoverDatasetSnapshot(s)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "complete chain" && !repair {
				t.Fatal("damaged/forked journal not retained")
			}
			dataset, err := store.Import([]byte(portable))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, got := range dataset.Transactions {
				if got.ID == f.row.ID && got.Desc == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("wrong replay prefix, wanted %q", want)
			}
			var copy recordRecovery
			_ = json.Unmarshal([]byte(snapshotRecoveryCopy(s)), &copy)
			if scenario == "fork without head" && (copy.Records[prefix+"0"] == "" || copy.Records[recordWorkspacePrefix(s.workspace)+"fork:0"] == "") {
				t.Fatal("fork records lost from downloadable copy")
			}
		})
	}
}

func TestJRNLOldQuarantineAutomaticallyRestoresOnlySample(t *testing.T) {
	for _, scenario := range []string{"sample", "edited sample copy", "own household", "stale sample caller", "foreign workspace", "foreign owner", "ambiguous households"} {
		t.Run(scenario, func(t *testing.T) {
			f := jrnlFixture(t)
			original := jrnlRead(t)
			copy := snapshotRecoveryCopy(original)
			var bundle recordRecovery
			_ = json.Unmarshal([]byte(copy), &bundle)
			bundle.Generation = "" // pre-JRNL copies did not retain generation.
			if scenario == "foreign workspace" {
				bundle.Workspace = "other-workspace"
			}
			if scenario == "foreign owner" {
				bundle.Owner = "other-owner"
			}
			if scenario == "edited sample copy" {
				portable, _, err := recoverDatasetSnapshot(original)
				if err != nil {
					t.Fatal(err)
				}
				ds, err := store.Import([]byte(portable))
				if err != nil {
					t.Fatal(err)
				}
				if ds.KV == nil {
					ds.KV = map[string]string{}
				}
				ds.KV["cashflux:sampleActive"] = "1"
				data, _ := store.Export(ds)
				bundle.Checkpoint, bundle.Head, bundle.Records = encodeRecordCheckpoint(data), "", nil
			}
			raw, _ := json.Marshal(bundle)
			const key = "cashflux:quarantine:1792418369"
			data := jrnlDataset()
			data.Transactions = data.Transactions[:1]
			data.KV = map[string]string{"cashflux:sampleActive": "1"}
			if scenario == "own household" || scenario == "stale sample caller" {
				delete(data.KV, "cashflux:sampleActive")
			}
			portable, _ := store.Export(data)
			values := map[string]string{key: string(raw), datasetStoreKey: encodeRecordCheckpoint(portable), datasetHeadKey: "", datasetGenKey: "sample-after-old-quarantine", datasetCheckpointBackupKey: "", datasetRecoverySourceKey: ""}
			if scenario == "ambiguous households" {
				other := store.EmptyDataset()
				other.KV = map[string]string{"different": "household"}
				rawData, _ := store.Export(other)
				otherCopy := recordRecovery{SchemaVersion: store.SchemaVersion + 1, Format: "v1", Workspace: original.workspace, Checkpoint: encodeRecordCheckpoint(rawData)}
				encoded, _ := json.Marshal(otherCopy)
				values[key+"-1"] = string(encoded)
			}
			solp22Batch(t, values, nil)
			loaded := jrnlApp(t)
			appstate.Default = loaded
			hydrateDataset()
			if scenario == "stale sample caller" {
				// Another instance may replace the sample between the caller's
				// sample check and this freshly read durable snapshot.
				uistate.SetSampleActive(true)
				if tryRecoverQuarantinedDataset(loaded, jrnlRead(t)) {
					t.Fatal("stale sample flag overwrote the durable household")
				}
			}
			if scenario == "sample" || scenario == "edited sample copy" {
				jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
				if !hydrationSucceeded || datasetQuarantined || browserstore.GetString(datasetRecoverySourceKey) != key {
					t.Fatal("quarantine restore did not complete durably")
				}
			} else if browserstore.GetString(datasetRecoverySourceKey) != "" {
				t.Fatal("automatic restore overwrote live/foreign/ambiguous data")
			}
			got, _, err := browserstore.GetSecure(key)
			if err != nil || got != string(raw) {
				t.Fatal("quarantine copy altered or deleted")
			}
		})
	}
}

func TestJRNLRepairFailureDoesNotQuarantineOrAcknowledge(t *testing.T) {
	f := jrnlFixture(t)
	solp22Batch(t, map[string]string{datasetGenKey: "split-before-failure"}, nil)
	s := jrnlRead(t)
	// Fail the first repair put, after snapshot authentication has succeeded.
	// No reader can see a partially repaired head or lose its recovery source.
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	loaded := jrnlApp(t)
	appstate.Default = loaded
	if err := hydrateDurableDataset(loaded, s); err == nil {
		t.Fatal("failed repair acknowledged")
	}
	if datasetQuarantined {
		t.Fatal("storage failure quarantined an authenticated household")
	}
	latest := jrnlRead(t)
	if latest.Values[datasetStoreKey] != f.checkpoint || latest.Values[datasetHeadKey] != s.Values[datasetHeadKey] || latest.Values[datasetGenKey] != "split-before-failure" {
		t.Fatal("failed repair changed saved data")
	}
	if err := hydrateDurableDataset(loaded, latest); err != nil {
		t.Fatal(err)
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
}

func TestJRNLRecoveryDownloadsAndRestoreHaveTheSameScopeGuard(t *testing.T) {
	for _, test := range []struct {
		name, workspace, owner string
		hosted, want           bool
	}{
		{"same local scope", "workspace", "", false, true},
		{"same account scope", "workspace", "owner", true, true},
		{"different workspace", "other", "owner", false, false},
		{"different owner", "workspace", "other", false, false},
		{"unknown hosted owner", "workspace", "", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := recordRecovery{Format: "v1", Workspace: "workspace", Owner: test.owner}
			currentOwner := test.owner
			if test.name == "different owner" {
				currentOwner = "owner"
			}
			if got := recoveryCopyMatchesScope(copy, test.workspace, currentOwner, test.hosted); got != test.want {
				t.Fatalf("scope allowed=%v want=%v", got, test.want)
			}
		})
	}
}

func TestJRNLFailedBackupRepairPausesWithoutQuarantine(t *testing.T) {
	f := jrnlFixture(t)
	solp22Batch(t, map[string]string{datasetStoreKey: "", datasetCheckpointBackupKey: encodeCheckpointBackup(recordWorkspace(), f.checkpoint)}, nil)
	loaded := jrnlApp(t)
	appstate.Default = loaded
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	hydrateDataset()
	if datasetQuarantined || quarantinePending || hydrationSucceeded || !autosaveBlocked() {
		t.Fatal("failed backup repair quarantined or acknowledged an authenticated household")
	}
	if browserstore.GetString(datasetStoreKey) != "" || browserstore.GetString(datasetHeadKey) == "" {
		t.Fatal("failed repair changed its original source")
	}
	hydrateDataset()
	if !hydrationSucceeded || datasetQuarantined {
		t.Fatal("backup repair did not recover on retry")
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
}

func TestJRNLWipeRetainsEveryRecoveryCopy(t *testing.T) {
	f := jrnlFixture(t)
	const quarantineKey = "cashflux:quarantine:1792418369"
	const recoveryKey = journalRecoveryPrefix + "wipe-retained"
	quarantine, journal := "exact original quarantine bytes", snapshotRecoveryCopy(jrnlRead(t))
	solp22Batch(t, map[string]string{quarantineKey: quarantine, recoveryKey: journal}, nil)
	if err := f.app.Wipe(); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) { wipeFinancialLocalState(func() { done(nil) }) }); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{quarantineKey: quarantine, recoveryKey: journal} {
		got, have, err := browserstore.GetSecure(key)
		if err != nil || !have || got != want {
			t.Fatalf("wipe deleted recovery copy %s: %v", key, err)
		}
	}
	raw, _, err := checkpointDatasetJSON([]byte(browserstore.GetString(datasetStoreKey)))
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := store.Import(raw)
	if err != nil || len(dataset.Transactions) != 0 || len(dataset.Accounts) != 0 {
		t.Fatal("financial wipe did not commit")
	}
}

func TestJRNLCheckpointRootGenerationMismatchRequiresRepair(t *testing.T) {
	data, err := store.Export(store.EmptyDataset())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := encodeRecordCheckpointGeneration(data, "checkpoint-generation")
	s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: map[string]string{datasetStoreKey: checkpoint, datasetHeadKey: "", datasetGenKey: "separate-generation-stamp"}, Errors: map[string]error{}}, "workspace"}
	portable, repair, err := recoverDatasetSnapshot(s)
	if err != nil || !repair {
		t.Fatalf("root generation mismatch was accepted for append: repair=%v err=%v", repair, err)
	}
	if _, err := store.Import([]byte(portable)); err != nil {
		t.Fatal(err)
	}
	s.Values[datasetGenKey] = "checkpoint-generation"
	if _, repair, err := recoverDatasetSnapshot(s); err != nil || repair {
		t.Fatalf("consistent empty journal needed repair: %v %v", repair, err)
	}
}

func TestJRNLForeignCheckpointCannotBeRelabeledAsActiveWorkspace(t *testing.T) {
	data, err := store.Export(jrnlDataset())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := encodeRecordCheckpointGeneration(data, "other-root")
	head, _ := json.Marshal(datasetRecordHead{Version: 1, Workspace: "other-workspace", BaseHash: checkpointHash(checkpoint), Generation: "other-root", Tail: recordWorkspacePrefix("other-workspace") + "old:batch"})
	s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: map[string]string{datasetStoreKey: checkpoint, datasetHeadKey: string(head), datasetGenKey: "other-root"}, Errors: map[string]error{}}, "active-workspace"}
	portable, _, err := recoverDatasetSnapshot(s)
	if !errors.Is(err, appstate.ErrStaleDataset) || portable != "" {
		t.Fatalf("foreign household relabeled as active: bytes=%d err=%v", len(portable), err)
	}
}

// The outer schema is a journal-reader ceiling; the inner dataset is migrated
// independently. Keep pre-bump checkpoints readable without admitting a future
// journal format or an invalid ceiling.
func TestJRNLCheckpointSchemaCompatibilityCeiling(t *testing.T) {
	data, err := store.Export(store.EmptyDataset())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name            string
		schema, version int
		wantError       bool
	}{
		{"older ceiling", store.SchemaVersion, 1, false},
		{"current ceiling", store.SchemaVersion + 1, 1, false},
		{"future ceiling", store.SchemaVersion + 2, 1, true},
		{"zero ceiling", 0, 1, true},
		{"negative ceiling", -1, 1, true},
		{"future format", store.SchemaVersion + 1, 2, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"schemaVersion":%d,"cashfluxRecordCheckpoint":%d,"dataset":%s}`, test.schema, test.version, data)
			decoded, wrapped, err := checkpointDatasetJSON([]byte(raw))
			if !wrapped || (err != nil) != test.wantError {
				t.Fatalf("wrapped=%v err=%v wantError=%v", wrapped, err, test.wantError)
			}
			if !test.wantError && string(decoded) != string(data) {
				t.Fatal("checkpoint changed the inner dataset")
			}
		})
	}
}
