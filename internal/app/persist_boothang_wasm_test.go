// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func bootAwait(t *testing.T, done <-chan struct{}) { bootAwaitLimit(t, done, 8*time.Second) }

func bootAwaitLimit(t *testing.T, done <-chan struct{}, limit time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(limit):
		stack := make([]byte, 1<<20)
		t.Fatalf("BOOT hydration did not finish within %s\n%s", limit, stack[:runtime.Stack(stack, true)])
	}
}

// A durable completion can resume boot (or recovery) while other writes are
// queued. Repair must not wait on the queue worker delivering that completion.
func TestBOOTQueuedCompletionHydration(t *testing.T) {
	f := jrnlFixture(t)
	s := jrnlRead(t)
	copy := snapshotRecoveryCopy(s)
	solp22Batch(t, map[string]string{
		quarantinePrefix + "boot":        copy,
		quarantinePrefix + "boot:reason": "previous failed boot",
		journalRecoveryPrefix + "boot":   copy,
		datasetGenKey:                    "interrupted-boot-generation",
	}, nil)
	loaded := jrnlApp(t)
	appstate.Default = loaded
	done := make(chan struct{}, 1)
	browserstore.SetSecure("cashflux:boot-pending", "queued", func(err error) {
		if err == nil {
			hydrateDataset()
		}
		done <- struct{}{}
	})
	browserstore.RemoveSecure("cashflux:boot-obsolete", nil)
	bootAwait(t, done)
	if !hydrationSucceeded || autosaveBlocked() {
		t.Fatal("boot failed to recover the queued repair")
	}
	jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
	raw, have, err := browserstore.GetSecure(quarantinePrefix + "boot")
	var retained recordRecovery
	if err != nil || !have || json.Unmarshal([]byte(raw), &retained) != nil || retained.Checkpoint != s.Values[datasetStoreKey] {
		t.Fatal("previous quarantine copy was lost")
	}
}

func TestBOOTHydrationRecoveryCombinations(t *testing.T) {
	f := jrnlFixture(t)
	baseline := jrnlRead(t)
	copy := snapshotRecoveryCopy(baseline)
	for _, quarantine := range []bool{false, true} {
		for _, recovery := range []bool{false, true} {
			for _, repair := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					name := fmt.Sprintf("quarantine=%v/recovery=%v/repair=%v/pending=%v", quarantine, recovery, repair, pending)
					t.Run(name, func(t *testing.T) {
						values := map[string]string{}
						for key, raw := range baseline.Values {
							values[key] = raw
						}
						values[quarantinePrefix+"1792368000"] = ""
						values[journalRecoveryPrefix+"boot-matrix"] = ""
						if quarantine {
							values[quarantinePrefix+"1792368000"] = copy
						}
						if recovery {
							values[journalRecoveryPrefix+"boot-matrix"] = copy
						}
						if repair {
							values[datasetGenKey] = "interrupted-matrix-generation"
						}
						solp22Batch(t, values, nil)
						loaded := jrnlApp(t)
						appstate.Default = loaded
						if pending {
							browserstore.Set("cashflux:boot-pending", "queued")
						}
						done := make(chan struct{}, 1)
						go func() { hydrateDataset(); done <- struct{}{} }()
						bootAwait(t, done)
						if !hydrationSucceeded || autosaveBlocked() {
							t.Fatal("hydration did not finish")
						}
						jrnlAssertRow(t, loaded, f.row.ID, f.row.Desc)
					})
				}
			}
		}
	}
}

func TestBOOTRepairDeadlineRejectsLateQueuedWrite(t *testing.T) {
	f := jrnlFixture(t)
	solp22Batch(t, map[string]string{datasetGenKey: "interrupted-deadline-generation"}, nil)
	snapshot := jrnlRead(t)
	loaded := jrnlApp(t)
	appstate.Default = loaded
	hydrationSucceeded, hadLocalDataset = false, false
	oldLimit := hydrationWaitLimit
	hydrationWaitLimit = 2 * time.Second
	defer func() { hydrationWaitLimit = oldLimit }()
	// Hold a conflicting readwrite transaction while repair is queued. Restore it
	// after the caller's deadline; the expired repair must never start a write.
	blocker := js.Global().Get("Function").New(`
 const data=__fakeIDB;
 return new Promise(resolve=>{
  const req=indexedDB.open("cashflux-kv",1);
  req.onsuccess=()=>{
   const db=req.result;
   const tx=db.transaction("kv","readwrite");
   tx._step=()=>{};
   resolve(tx);
  };
 });`).Invoke()
	blocked := make(chan js.Value, 1)
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any { blocked <- args[0]; return nil })
	blocker.Call("then", callback)
	tx := <-blocked
	callback.Release()
	result := make(chan error, 1)
	go func() { result <- hydrateDurableDataset(loaded, snapshot) }()
	var err error
	queued := false
	deadline := time.After(4 * time.Second)
waiting:
	for {
		queued = queued || tx.Get("_db").Get("_txQueue").Length() > 0
		select {
		case err = <-result:
			break waiting
		case <-deadline:
			tx.Call("abort")
			stack := make([]byte, 1<<20)
			t.Fatalf("BOOT repair deadline did not return\n%s", stack[:runtime.Stack(stack, true)])
		case <-time.After(5 * time.Millisecond):
		}
	}
	tx.Call("abort")
	// The short limit tests the blocked repair only. Postconditions read the
	// complete accumulated fixture with the normal production deadline.
	hydrationWaitLimit = oldLimit
	if !queued {
		t.Error("repair did not reach the pending transaction queue")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("repair deadline = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := browserstore.BarrierContext(ctx); err != nil {
		t.Fatal(err)
	}
	after := jrnlRead(t)
	if after.Values[datasetStoreKey] != snapshot.Values[datasetStoreKey] || after.Values[datasetGenKey] != snapshot.Values[datasetGenKey] || hydrationSucceeded || !autosaveBlocked() {
		t.Fatal("late repair acquired write authority or replaced saved records")
	}
	portable, _, err := recoverDatasetSnapshot(after)
	if err != nil {
		t.Fatal(err)
	}
	solp22AssertRow(t, portable, f.row)
}

func TestBOOTProfile3292WithRetainedCopiesAndQueuedWrite(t *testing.T) {
	f := jrnlFixture(t)
	ds := store.SampleDataset()
	for len(ds.Accounts) < 15 {
		account := ds.Accounts[0]
		account.ID = fmt.Sprintf("boot-account-%d", len(ds.Accounts))
		ds.Accounts = append(ds.Accounts, account)
	}
	ds.Accounts = ds.Accounts[:15]
	ds.Transactions = make([]domain.Transaction, 3292)
	for i := range ds.Transactions {
		row := f.row
		row.ID = fmt.Sprintf("boot-row-%04d", i)
		row.AccountID = ds.Accounts[i%len(ds.Accounts)].ID
		ds.Transactions[i] = row
	}
	if ds.KV == nil {
		ds.KV = map[string]string{}
	}
	ds.KV["cashflux:sampleActive"] = "0"
	input, err := store.Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.app.ImportJSON(input); err != nil {
		t.Fatal(err)
	}
	raw, err := f.app.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := encodeRecordCheckpointGeneration(raw, "boot-profile-root")
	solp22Batch(t, map[string]string{datasetStoreKey: checkpoint, datasetHeadKey: "", datasetGenKey: "boot-profile-root"}, nil)
	hydratedRecordApp, hydratedRecordRevision = f.app, f.app.Rev()
	hydratedRecordCursor, _ = f.app.Store().ChangeCursor()
	hydratedDatasetState.app = nil
	datasetMyGen = "boot-profile-root"
	newDatasetAutosave(f.app, browserstore.SetSecureBatch, nil)
	row := f.app.Transactions()[0]
	row.Desc = "profile journal edit"
	if err := f.app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) { f.app.RequestDurable(func(result appstate.DurableResult) { done(result.Err) }) }); err != nil {
		t.Fatal(err)
	}
	snapshot := jrnlRead(t)
	if snapshot.Values[datasetHeadKey] == "" {
		t.Fatal("profile fixture has no real journal head")
	}
	copy := snapshotRecoveryCopy(snapshot)
	solp22Batch(t, map[string]string{quarantinePrefix + "1792368001": copy, journalRecoveryPrefix + "profile": copy, datasetGenKey: "profile-interrupted"}, nil)
	// Include the profile's record count and retained journal tombstones. They
	// remain authenticated snapshot inputs, rather than bypassing storage.
	count := len(browserstore.Keys())
	for count < 415 {
		padding := map[string]string{}
		for len(padding) < 32 && count < 415 {
			padding[fmt.Sprintf("%sboot-inactive:%d", recordWorkspacePrefix(snapshot.workspace), count)] = ""
			count++
		}
		solp22Batch(t, padding, nil)
	}

	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(oldLogger)
	t.Cleanup(func() { t.Log(logs.String()) })
	loaded, err := appstate.New(&logs, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { loaded.SetDurableWriter(nil); loaded.Store().Close() })
	appstate.Default = loaded
	done := make(chan struct{}, 1)
	started := time.Now()
	browserstore.SetSecure("cashflux:boot-pending", "profile-queued", func(err error) {
		if err == nil {
			hydrateDataset()
		}
		done <- struct{}{}
	})
	bootAwaitLimit(t, done, 30*time.Second)
	if !hydrationSucceeded || autosaveBlocked() || len(loaded.Transactions()) != 3292 || len(loaded.Accounts()) != 15 {
		t.Fatal("profile boot lost data or stayed paused")
	}
	jrnlAssertRow(t, loaded, row.ID, row.Desc)
	for _, stage := range []string{"snapshot read", "verify", "replay", "repair", "import"} {
		if !strings.Contains(logs.String(), stage) {
			t.Errorf("missing hydration stage log %q", stage)
		}
	}
	t.Logf("profile hydration completed in %s with 15 accounts, 3292 transactions, journal, retained copies, repair and queued write", time.Since(started))
}
