// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
)

// A wipe intentionally visits all financial keys, including earlier tests' keys.
// Restore their exact values so this regression does not disturb the full lane.
func solp23PreserveStorage(t *testing.T) {
	t.Helper()
	values, transient := map[string]string{}, map[string]string{}
	values[seededFlagKey] = browserstore.GetString(seededFlagKey)
	for _, key := range perWorkspaceKeys {
		values[key] = browserstore.GetString(key)
	}
	for _, key := range browserstore.Keys() {
		if atrest.Classify(key) == atrest.ShouldNotPersist {
			transient[key] = browserstore.GetString(key)
		} else {
			values[key] = browserstore.GetString(key)
		}
	}
	t.Cleanup(func() {
		solp22Batch(t, values, nil)
		for key, value := range transient {
			browserstore.SetMemory(key, value)
		}
	})
}

// Fail a specific put in the real secure adapter's transaction. Choosing a late
// key proves rollback of preceding puts, rather than only failure before writing.
func solp23FailPut(t *testing.T, key string) *bool {
	t.Helper()
	req := js.Global().Get("indexedDB").Call("open", "solp23-hook")
	upgrade := js.FuncOf(func(js.Value, []js.Value) any {
		req.Get("result").Call("createObjectStore", "probe")
		return nil
	})
	ready := make(chan js.Value, 1)
	opened := js.FuncOf(func(js.Value, []js.Value) any {
		db := req.Get("result")
		objectStore := db.Call("transaction", "probe", "readonly").Call("objectStore", "probe")
		ready <- js.Global().Get("Object").Call("getPrototypeOf", objectStore)
		return nil
	})
	req.Set("onupgradeneeded", upgrade)
	req.Set("onsuccess", opened)
	var prototype js.Value
	select {
	case prototype = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("opening failure-injection fixture timed out")
	}
	upgrade.Release()
	opened.Release()
	original := prototype.Get("put")
	triggered := false
	failure := js.Global().Get("Function").New(`var err = new Error("SOLP23 interrupted write"); err.name = "QuotaExceededError"; throw err;`)
	hook := js.FuncOf(func(this js.Value, args []js.Value) any {
		if !triggered && len(args) > 1 && args[1].String() == key {
			triggered = true
			// Queue the failure at this exact put. failNextWrite would instead
			// fail the earliest already-queued put, missing partial-write rollback.
			return this.Get("_tx").Call("_enqueue", true, failure)
		}
		return original.Call("call", this, args[0], args[1])
	})
	prototype.Set("put", hook)
	t.Cleanup(func() {
		prototype.Set("put", original)
		hook.Release()
	})
	return &triggered
}

func TestSOLP23WipeJournalBoundary(t *testing.T) {
	isolateDurableGlobals(t)
	for _, mode := range []string{"aborted commit", "late interrupted commit", "stale generation", "success"} {
		t.Run(mode, func(t *testing.T) {
			fixture := solp22Journal(t)
			solp23PreserveStorage(t)
			const satellite = "cashflux:solp23-financial-satellite"
			solp22Batch(t, map[string]string{satellite: "private financial state"}, nil)
			t.Cleanup(func() {
				if err := enc31Wait(t, func(done func(error)) { browserstore.RemoveSecure(satellite, done) }); err != nil {
					t.Error(err)
				}
			})
			head, generation := browserstore.GetString(datasetHeadKey), readDatasetGen()
			var parsed datasetRecordHead
			if err := json.Unmarshal([]byte(head), &parsed); err != nil {
				t.Fatal(err)
			}
			record := browserstore.GetString(parsed.Tail)
			if err := fixture.app.Wipe(); err != nil {
				t.Fatal(err)
			}
			var failed *bool
			switch mode {
			case "aborted commit":
				js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
			case "late interrupted commit":
				failed = solp23FailPut(t, satellite)
			case "stale generation":
				datasetMyGen = id.New()
			}
			entitlement := datasetMyGen
			continued := false
			if mode == "success" {
				if err := enc31Wait(t, func(done func(error)) {
					wipeFinancialLocalState(func() { continued = true; done(nil) })
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				wipeFinancialLocalState(func() { continued = true })
				browserstore.Barrier()
			}
			if failed != nil && !*failed {
				t.Fatal("late transaction failure was not injected")
			}
			if mode != "success" {
				if continued || datasetMyGen != entitlement || !suspendAutosave {
					t.Fatal("failed wipe acknowledged, adopted a generation, or resumed autosave")
				}
				if browserstore.GetString(datasetStoreKey) != fixture.checkpoint || browserstore.GetString(datasetHeadKey) != head || readDatasetGen() != generation || browserstore.GetString(parsed.Tail) != record || browserstore.GetString(satellite) != "private financial state" {
					t.Fatal("failed wipe changed checkpoint, journal, generation, or satellite")
				}
				solp22Reload(t, fixture.row)
				return
			}
			if !continued || datasetMyGen == generation || readDatasetGen() != datasetMyGen || browserstore.GetString(datasetHeadKey) != "" || browserstore.GetString(satellite) != "" {
				t.Fatal("successful wipe did not commit empty head, new generation and satellite tombstones")
			}
			raw, _, err := materializeLocalDataset()
			if err != nil {
				t.Fatal(err)
			}
			ds, err := store.Import([]byte(raw))
			if err != nil || len(ds.Transactions) != 0 || len(ds.Accounts) != 0 || len(ds.Budgets) != 0 {
				t.Fatalf("wiped dataset retained financial rows: %v", err)
			}
		})
	}
}

func TestSOLP23StartupWorkspaceJournalBoundary(t *testing.T) {
	isolateDurableGlobals(t)
	for _, mode := range []string{"success", "blob save failure", "dataset commit failure", "registry adoption failure", "invalid target"} {
		t.Run(mode, func(t *testing.T) {
			fixture := solp22Journal(t)
			solp23PreserveStorage(t)
			const target = "solp23-startup-target"
			currentID := loadRegistry().ActiveID
			currentKey, targetKey := wsBlobKey(currentID), wsBlobKey(target)
			previous := map[string]string{currentKey: browserstore.GetString(currentKey), targetKey: browserstore.GetString(targetKey)}
			t.Cleanup(func() { solp22Batch(t, previous, nil) })
			ds := store.EmptyDataset()
			ds.SettingsKV = map[string]string{"solp23-target": "target workspace"}
			portable, err := store.Export(ds)
			if err != nil {
				t.Fatal(err)
			}
			blob, _ := json.Marshal(map[string]string{datasetStoreKey: string(portable), "cashflux:layout": "target layout"})
			r := loadRegistry().Add(target, "Startup target").SetStartup(target)
			registry, _ := json.Marshal(r)
			if mode == "invalid target" {
				blob = []byte("broken bundle")
			}
			solp22Batch(t, map[string]string{currentKey: "older portable recovery copy", targetKey: string(blob), workspacesKey: string(registry)}, nil)
			head, generation := browserstore.GetString(datasetHeadKey), readDatasetGen()
			// Boot has not hydrated SQLite yet. Make it empty to prove the old
			// workspace's blob is reconstructed from checkpoint + journal.
			hydrationSucceeded = false
			if err := fixture.app.Store().Load(store.EmptyDataset()); err != nil {
				t.Fatal(err)
			}
			var failed *bool
			switch mode {
			case "blob save failure":
				failed = solp23FailPut(t, currentKey)
			case "dataset commit failure":
				failed = solp23FailPut(t, datasetStoreKey)
			case "registry adoption failure":
				failed = solp23FailPut(t, workspacesKey)
			}
			applyStartupWorkspace()
			// No Barrier here: startup must await durability before returning
			// to its caller, which hydrates immediately afterwards.
			if failed != nil && !*failed {
				t.Fatal("startup transaction failure was not injected")
			}
			if mode != "success" {
				if browserstore.GetString(datasetStoreKey) != fixture.checkpoint || browserstore.GetString(datasetHeadKey) != head || readDatasetGen() != generation || datasetMyGen != generation || browserstore.GetString(workspacesKey) != string(registry) || browserstore.GetString(currentKey) != "older portable recovery copy" || browserstore.GetString(targetKey) != string(blob) {
					t.Fatal("failed startup switch changed the original workspace or recovery blob")
				}
				solp22Reload(t, fixture.row)
				return
			}
			if loadRegistry().ActiveID != target || datasetMyGen == generation || readDatasetGen() != datasetMyGen || browserstore.GetString(datasetHeadKey) != "" || browserstore.GetString(datasetStoreKey) != string(portable) || browserstore.GetString("cashflux:layout") != "target layout" {
				t.Fatal("startup returned before the complete target workspace was adopted")
			}
			var saved map[string]string
			if err := json.Unmarshal([]byte(browserstore.GetString(currentKey)), &saved); err != nil {
				t.Fatal(err)
			}
			solp22AssertRow(t, saved[datasetStoreKey], fixture.row)
			hydrateDataset()
			setting, _, err := fixture.app.Store().GetSettingKV("solp23-target")
			if !hydrationSucceeded || len(fixture.app.Transactions()) != 0 || err != nil || setting != "target workspace" {
				t.Fatal("hydration did not load the durably selected startup workspace")
			}
		})
	}
}

func TestSOLP23StartupRejectsPreparedStaleGeneration(t *testing.T) {
	isolateDurableGlobals(t)
	fixture := solp22Journal(t)
	generation := readDatasetGen()
	row := fixture.row
	row.Desc = "SOLP23 newer durable edit during startup preparation"
	if err := fixture.app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) {
		fixture.app.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
	}); err != nil {
		t.Fatal(err)
	}
	head, current := browserstore.GetString(datasetHeadKey), readDatasetGen()
	empty, err := store.Export(store.EmptyDataset())
	if err != nil {
		t.Fatal(err)
	}
	err = applyBundle(map[string]string{datasetStoreKey: string(empty)}, nil, map[string]string{datasetGenKey: generation})
	if !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("stale prepared startup replacement = %v", err)
	}
	if datasetMyGen != current || readDatasetGen() != current || browserstore.GetString(datasetHeadKey) != head || browserstore.GetString(datasetStoreKey) != fixture.checkpoint {
		t.Fatal("stale prepared startup switch changed the newer durable dataset")
	}
	solp22Reload(t, row)
}
