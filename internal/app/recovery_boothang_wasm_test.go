//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workspace"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestBOOTLargeRecoveryHistoryDoesNotBlockHydration(t *testing.T) {
	solp22Storage(t)
	paint := render.New(t)
	f := bootFreshHousehold(t)
	backup, haveBackup, err := browserstore.GetSecure(datasetCheckpointBackupKey)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	cb := js.FuncOf(func(_ js.Value, args []js.Value) any { done <- args[0].String(); return nil })
	defer cb.Release()
	instrument := js.Global().Get("Function").New("large", "done", `
 const request=indexedDB.open("cashflux-kv",1);
 const state={values:0,all:0,restore:()=>{}};
 request.onerror=()=>done("could not open fixture");
 request.onsuccess=()=>{
  const database=request.result;
  const tx=database.transaction("kv","readwrite",{durability:"strict"});
  const store=tx.objectStore("kv");
  for(let i=0;i<126;i++)store.put(large,"cashflux:stale-recovery:boot:"+i);
  for(let i=0;i<8;i++)store.put(large,"cashflux:quarantine:"+(1700000000+i));
  store.put(large,"cashflux:dataset:checkpoint-backup");
  store.put(large,"cashflux:journal-recovery:boot");
  tx.onabort=()=>done("could not seed fixture");
  tx.oncomplete=()=>{
   const prototype=Object.getPrototypeOf(database),original=prototype.transaction;
   prototype.transaction=function(...args){
    const tx=original.apply(this,args);
    if(this.name!=="cashflux-kv")return tx;
    const open=tx.objectStore;
    tx.objectStore=function(...a){return new Proxy(open.apply(tx,a),{get(t,k){
     if(k==="get")return key=>{if(/^cashflux:(stale-recovery:|quarantine:|journal-recovery:|dataset:checkpoint-backup$)/.test(key))state.values++;return t.get(key)};
     if(k==="getAll")return (...x)=>{state.all++;return t.getAll(...x)};
     const v=t[k];return typeof v==="function"?v.bind(t):v;
    }})};return tx;
   };
   state.restore=()=>{prototype.transaction=original;database.close()};done("");
  };
 };return state;
 `).Invoke(strings.Repeat("opaque old recovery bytes", 16384), cb)
	if err := <-done; err != "" {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		instrument.Call("restore")
		var keys []string
		for i := 0; i < 126; i++ {
			keys = append(keys, fmt.Sprintf("cashflux:stale-recovery:boot:%d", i))
		}
		for i := 0; i < 8; i++ {
			keys = append(keys, fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i))
		}
		keys = append(keys, datasetCheckpointBackupKey, journalRecoveryPrefix+"boot")
		jrnlRemoveFixtureRecords(t, keys)
		if haveBackup {
			solp22Batch(t, map[string]string{datasetCheckpointBackupKey: backup}, nil)
		}
	})
	var names []string
	for i := 0; i < 126; i++ {
		names = append(names, fmt.Sprintf("cashflux:stale-recovery:boot:%d", i))
	}
	for i := 0; i < 8; i++ {
		names = append(names, fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i))
	}
	names = append(names, datasetCheckpointBackupKey, journalRecoveryPrefix+"boot")
	browserstore.Reload(names...)
	loaded := jrnlApp(t)
	appstate.Default = loaded
	hydrateDataset()
	rows := loaded.Transactions()
	if len(rows) == 0 {
		t.Fatal("hydrated household has no transaction to paint")
	}
	paint.Render(Body(Div(ID("boot-financial-content"), rows[0].Desc)))
	paint.Stabilize()
	visible := false
	for _, node := range paint.AllByTag("div") {
		if node.Attr("id") == "boot-financial-content" && node.Text() == rows[0].Desc {
			visible = true
		}
	}
	if !visible {
		t.Fatal("financial content did not paint after hydration")
	}
	values, all := instrument.Get("values").Int(), instrument.Get("all").Int()
	if values != 0 || all != 0 {
		t.Fatalf("BOOT hydration read %d recovery values and %d full scans", values, all)
	}
	if !hydrationSucceeded || autosaveBlocked() {
		t.Fatal("boot did not admit the saved household")
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
}

func TestBOOTStaleWriterDeduplicatesTheSameHouseholdEdit(t *testing.T) {
	// Recovery copies survive fixture cleanup, so isolate this test by scope.
	workspaceID := "boot-stale-" + id.New()
	fixture := jrnlFixtureForWorkspace(t, workspaceID)
	owner := browserstore.GetString(datasetOwnerKey)
	recoveryScopePrefix := fmt.Sprintf("cashflux:stale-recovery:%s:%s:", checkpointHash(workspaceID), checkpointHash(owner))
	initial := map[string]bool{}
	for _, key := range browserstore.Keys() {
		initial[key] = true
	}
	t.Cleanup(func() {
		var remove []string
		for _, key := range browserstore.Keys() {
			if !initial[key] && strings.HasPrefix(key, recoveryScopePrefix) {
				remove = append(remove, key)
			}
		}
		jrnlRemoveFixtureRecords(t, remove)
	})
	var retained string
	for i := 0; i < 2; i++ {
		resetStaleDatasetState()
		preserveStaleDatasetEdits(fixture.app, recordWorkspace())
		deadline := time.Now().Add(15 * time.Second)
		for !staleRecoveryReady && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if !staleRecoveryReady {
			t.Fatal("stale edit was not durably retained")
		}
		if i == 0 {
			retained = staleDatasetRecoveryKey
		} else if staleDatasetRecoveryKey != retained {
			t.Fatal("same stale edit produced a second recovery key")
		}
	}
	count := 0
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, recoveryScopePrefix) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("same stale edit left %d retained copies in its isolated scope, want one", count)
	}
	if !strings.HasPrefix(retained, recoveryScopePrefix) {
		t.Fatalf("retained stale edit key %q is outside its isolated scope", retained)
	}
	raw, have, err := browserstore.GetSecure(retained)
	if err != nil || !have || !strings.Contains(raw, fixture.row.Desc) {
		t.Fatal("retained edit cannot be downloaded")
	}
}

// The boot regression needs its own single render fixture, rather than the
// autosave fixture which already owns testkit's process-global mock DOM.
func bootFreshHousehold(t *testing.T) solp22Fixture {
	t.Helper()
	a := jrnlApp(t)
	oldApp, oldHad, oldReady, oldGen := appstate.Default, hadLocalDataset, hydrationSucceeded, datasetMyGen
	oldQuarantined, oldPending, oldRaw, oldAvailable := datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable
	oldHosted := currentHostedHydration()
	oldHydrated := hydratedDatasetState
	oldRecordApp, oldCursor, oldRevision := hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision
	oldEnvelope := pendingEnvelopeRaw
	keys := []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey, datasetCheckpointBackupKey, datasetRecoverySourceKey}
	previous := map[string]string{}
	absent := []string{}
	for _, key := range keys {
		raw, have, err := browserstore.GetSecure(key)
		if err != nil {
			t.Fatal(err)
		}
		if have {
			previous[key] = raw
		} else {
			absent = append(absent, key)
		}
	}
	t.Cleanup(func() {
		solp22Batch(t, previous, nil)
		jrnlRemoveFixtureRecords(t, absent)
		appstate.Default, hadLocalDataset, hydrationSucceeded, datasetMyGen = oldApp, oldHad, oldReady, oldGen
		datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = oldQuarantined, oldPending, oldRaw, oldAvailable
		setHostedHydration(oldHosted)
		hydratedDatasetState = oldHydrated
		hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = oldRecordApp, oldCursor, oldRevision
		pendingEnvelopeRaw = oldEnvelope
	})
	dataset := store.SampleDataset()
	dataset.Transactions = dataset.Transactions[:2]
	if dataset.KV == nil {
		dataset.KV = map[string]string{}
	}
	dataset.KV["cashflux:sampleActive"] = "0"
	if err := a.Store().Load(dataset); err != nil {
		t.Fatal(err)
	}
	data, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	scope, generation := "boothang-"+id.New(), id.New()
	registry, _ := json.Marshal(workspace.Registry{}.Add(scope, "Boot regression"))
	checkpoint := encodeRecordCheckpointGeneration(data, generation)
	solp22Batch(t, map[string]string{datasetStoreKey: checkpoint, datasetHeadKey: "", datasetGenKey: generation, datasetOwnerKey: "", workspacesKey: string(registry), datasetCheckpointBackupKey: "", datasetRecoverySourceKey: ""}, nil)
	appstate.Default = a
	datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = false, false, "", false
	pendingEnvelopeRaw = ""
	return solp22Fixture{app: a, row: dataset.Transactions[0], checkpoint: checkpoint}
}

func TestBOOTLegacyColdBackupRecoversDamagedOrMissingCheckpoint(t *testing.T) {
	for _, live := range []string{"damaged checkpoint", ""} {
		t.Run(live, func(t *testing.T) {
			solp22Storage(t)
			render.New(t)
			fixture := bootFreshHousehold(t)
			enc31RawRecord(t, datasetCheckpointBackupKey, encodeCheckpointBackup(recordWorkspace(), fixture.checkpoint, ""))
			solp22Batch(t, map[string]string{datasetStoreKey: live}, nil)
			loaded := jrnlApp(t)
			appstate.Default = loaded
			hydrateDataset()
			if !hydrationSucceeded || autosaveBlocked() {
				t.Fatal("cold legacy checkpoint backup did not recover")
			}
			jrnlAssertRow(t, loaded, fixture.row.ID, fixture.row.Desc)
		})
	}
}

func TestBOOTRepairLeavesExistingJournalTombstonesUnchanged(t *testing.T) {
	f := jrnlFixture(t)
	key := recordWorkspacePrefix(recordWorkspace()) + "boot-empty-pad"
	solp22Batch(t, map[string]string{key: "", datasetGenKey: "interrupted-with-empty-pad"}, nil)
	before, err := browserstore.ReadSecureSnapshotContext(context.Background(), []string{key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, have := before.EncodedRecord(key)
	if !have {
		t.Fatal("fixture tombstone absent")
	}
	loaded := jrnlApp(t)
	appstate.Default = loaded
	hydrateDataset()
	if !hydrationSucceeded {
		t.Fatal("journal repair did not complete")
	}
	after, err := browserstore.ReadSecureSnapshotContext(context.Background(), []string{key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, have := after.EncodedRecord(key)
	if !have || raw != old {
		t.Fatal("repair rewrote an already empty journal record")
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
}

func TestBOOTRepairRetainsUnreadableBackupCiphertext(t *testing.T) {
	fixture := jrnlFixture(t)
	initial := map[string]bool{}
	for _, key := range browserstore.Keys() {
		initial[key] = true
	}
	const damaged = `{"format":"cashflux-atrest-v1","ciphertext":"broken"}`
	enc31RawRecord(t, datasetCheckpointBackupKey, damaged)
	solp22Batch(t, map[string]string{datasetGenKey: "repair-with-damaged-backup"}, nil)
	loaded := jrnlApp(t)
	appstate.Default = loaded
	hydrateDataset()
	if !hydrationSucceeded {
		t.Fatal("valid live journal was blocked by its damaged backup")
	}
	retained := false
	for _, key := range browserstore.Keys() {
		if initial[key] || !strings.HasPrefix(key, journalRecoveryPrefix) {
			continue
		}
		raw, have, err := browserstore.GetSecure(key)
		if err != nil || !have {
			t.Fatal(err)
		}
		var copy recordRecovery
		if json.Unmarshal([]byte(raw), &copy) == nil && copy.ProtectedRecords[datasetCheckpointBackupKey] == damaged {
			retained = true
		}
	}
	if !retained {
		t.Fatal("unreadable backup was replaced without retaining its exact ciphertext")
	}
	jrnlAssertRow(t, loaded, fixture.row.ID, fixture.row.Desc)
}
