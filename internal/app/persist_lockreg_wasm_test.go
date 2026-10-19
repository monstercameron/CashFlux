// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func lockregDurable(t *testing.T, a *appstate.App) {
	t.Helper()
	if err := enc31Wait(t, func(done func(error)) {
		a.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
	}); err != nil {
		t.Fatalf("durable save: %v (current app=%t writer app=%t snapshot app=%t my generation=%q saved generation=%q suspended=%t stale=%t workspace=%q)", err,
			appstate.Default == a, hydratedRecordApp == a, hydratedDatasetState.app == a, datasetMyGen, readDatasetGen(), suspendAutosave, staleDatasetBlocked, recordWorkspace())
	}
	if staleDatasetBlocked {
		t.Fatal("this page's own App Lock checkpoint triggered the stale-tab overlay")
	}
}

func TestLOCKREGFreshAppLockBudgetSurvivesReloadAndUnlock(t *testing.T) {
	f := jrnlFixture(t)
	enc31FakeDocument(t)
	oldResave, oldClosing := resaveDataset, appLockClosing
	t.Cleanup(func() { resaveDataset, appLockClosing = oldResave, oldClosing })

	// Model first boot: the sample is in SQLite, but this App has not imported
	// a persisted checkpoint. A successful first autosave must establish the
	// same writer authority as an imported household.
	values := map[string]string{datasetStoreKey: "", datasetHeadKey: "", datasetGenKey: "", datasetCheckpointBackupKey: ""}
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, recordWorkspacePrefix(jrnlFixtureWorkspace)) {
			values[key] = ""
		}
	}
	solp22Batch(t, values, nil)
	hydratedDatasetState.app = nil
	hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = nil, 0, 0
	hadLocalDataset = false
	_, resaveDataset = newDatasetAutosave(f.app, browserstore.SetSecureBatch, nil)
	lockregDurable(t, f.app)

	const passcode = "cashflux77"
	suspendAutosave = true
	config := applock.Config{}.WithPasscode(passcode, "lockreg-salt", 0, "")
	if err := runSecureLockTransition(passcode, config, false); err != nil {
		t.Fatal(err)
	}
	suspendAutosave = false
	// This is the immediate force-save issued by finishAppLockEnable, which
	// used to reject the transition's new generation before the first click.
	migrateDatasetAtRest()
	lockregDurable(t, f.app)

	p := prefs.Default()
	p.BudgetIncomeMode, p.MonthlyIncomeMinor = "fixed", 600000
	if err := uistate.PersistPrefs(p); err != nil {
		t.Fatal(err)
	}
	lockregDurable(t, f.app)
	if browserstore.GetString(datasetHeadKey) == "" {
		t.Fatal("budget setting did not reach the durable journal")
	}

	// A reload discards SQLite and the passcode and destroys the unlocked WDK.
	// The Node-only IndexedDB fixture retains the encrypted checkpoint/journal.
	f.app.SetDurableWriter(nil)
	if err := enc31Wait(t, func(done func(error)) { browserstore.LockSecureThen(func() { done(nil) }) }); err != nil {
		t.Fatal(err)
	}
	loaded := jrnlApp(t)
	appstate.Default = loaded
	activePasscode, pendingEnvelopeRaw = "", ""
	hydrationSucceeded, hadLocalDataset, suspendAutosave = false, false, true
	hydratedDatasetState.app, hydratedRecordApp = nil, nil
	continued := make(chan struct{}, 1)
	resumeProtectedBoot(atrest.ErrLocked, showProtectedStorageGate, func() {
		hydrateDataset()
		suspendAutosave = false
		continued <- struct{}{}
	})
	doc := js.Global().Get("document")
	if !appLockActive() || !doc.Call("getElementById", "app").Get("inert").Bool() || hydrationSucceeded {
		t.Fatal("locked reload did not gate the empty app")
	}
	// A wrong passcode must keep both the gate and the empty store intact.
	if err := enc31Wait(t, func(done func(error)) { unlockProtectedSession("wrong-passcode", false, done) }); err == nil {
		t.Fatal("wrong passcode unlocked the dataset")
	}
	if !appLockActive() || len(loaded.Accounts()) != 0 {
		t.Fatal("failed unlock released or hydrated the app")
	}
	doc.Call("getElementById", "cf-applock-input").Set("value", passcode)
	doc.Call("getElementById", "cf-protected-unlock").Call("click")
	select {
	case <-continued:
	case <-time.After(30 * time.Second):
		t.Fatal("unlock did not resume dataset hydration")
	}
	if !hydrationSucceeded || !hadLocalDataset || appLockActive() || doc.Call("getElementById", "app").Get("inert").Bool() {
		t.Fatal("unlock left the hydrated app gated or inert")
	}
	got := uistate.LoadPrefs()
	if got.BudgetIncomeMode != "fixed" || got.MonthlyIncomeMinor != 600000 {
		t.Fatalf("budget income after unlock = %q, %d", got.BudgetIncomeMode, got.MonthlyIncomeMinor)
	}
	newDatasetAutosave(loaded, browserstore.SetSecureBatch, nil)
	got.MonthlyIncomeMinor = 610000
	if err := uistate.PersistPrefs(got); err != nil {
		t.Fatal(err)
	}
	lockregDurable(t, loaded)
	if _, _, err := materializeLocalDataset(); err != nil {
		t.Fatalf("unlocked budget could not be saved: %v", err)
	}
}

func TestLOCKREGAppLockAdoptsUnchangedLegacyCheckpoint(t *testing.T) {
	f := jrnlFixture(t)
	suspendAutosave = true
	// Older installations can load portable JSON without a record wrapper.
	// Enabling App Lock replaces its generation but keeps these exact bytes:
	// encryption is already provided by the outer WDK record boundary.
	checkpoint, err := f.app.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{datasetStoreKey: string(checkpoint), datasetHeadKey: "", datasetCheckpointBackupKey: ""}
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, recordWorkspacePrefix(jrnlFixtureWorkspace)) {
			values[key] = ""
		}
	}
	solp22Batch(t, values, nil)
	hydratedDatasetState.app = nil
	newDatasetAutosave(f.app, browserstore.SetSecureBatch, nil)
	before := readDatasetGen()
	config := applock.Config{}.WithPasscode("cashflux77", "lockreg-legacy-salt", 0, "")
	if err := runSecureLockTransition("cashflux77", config, false); err != nil {
		t.Fatal(err)
	}
	suspendAutosave = false
	if browserstore.GetString(datasetStoreKey) != string(checkpoint) || readDatasetGen() == before {
		t.Fatal("fixture did not replace the generation with identical checkpoint bytes")
	}
	lockregDurable(t, f.app)
}

func TestLOCKREGPausedWriterWaitsForTransitionAdoption(t *testing.T) {
	for _, queued := range []bool{false, true} {
		name := "requested during transition"
		if queued {
			name = "queued before transition"
		}
		t.Run(name, func(t *testing.T) {
			f := jrnlFixture(t)
			row := f.row
			row.Desc = "pending edit before App Lock transition"
			if err := f.app.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			checkpoint, err := f.app.ExportJSONRedacted()
			if err != nil {
				t.Fatal(err)
			}
			completed := make(chan error, 1)
			request := func() { f.app.RequestDurable(func(result appstate.DurableResult) { completed <- result.Err }) }
			if queued {
				request()
			}
			// A transition publishes its durable cache before its completion
			// goroutine adopts the generation. Autosave is suspended throughout.
			suspendAutosave = true
			solp22Batch(t, map[string]string{
				datasetStoreKey: string(checkpoint), datasetHeadKey: "", datasetGenKey: "lockreg-transition-generation",
			}, nil)
			if !queued {
				request()
			}
			select {
			case err = <-completed:
			case <-time.After(30 * time.Second):
				t.Fatal("paused save was not completed")
			}
			if err == nil || errors.Is(err, appstate.ErrStaleDataset) || staleDatasetBlocked {
				t.Fatalf("paused transition = %v, stale=%t; want a retryable pause", err, staleDatasetBlocked)
			}
			datasetMyGen = "lockreg-transition-generation"
			suspendAutosave = false
			lockregDurable(t, f.app)
		})
	}
}

func TestLOCKREGWriterStillRejectsUnadoptedGeneration(t *testing.T) {
	f := jrnlFixture(t)
	// A successful local commit establishes writer authority, but does not
	// entitle it to adopt another page's later generation, even with identical
	// checkpoint bytes. That page never advanced this writer's datasetMyGen.
	checkpoint := browserstore.GetString(datasetStoreKey)
	solp22Batch(t, map[string]string{datasetGenKey: "lockreg-other-page-generation"}, nil)
	err := enc31Wait(t, func(done func(error)) {
		f.app.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
	})
	if !errors.Is(err, appstate.ErrStaleDataset) || !staleDatasetBlocked {
		t.Fatalf("foreign generation = %v, blocked = %v", err, staleDatasetBlocked)
	}
	if readDatasetGen() != "lockreg-other-page-generation" || browserstore.GetString(datasetStoreKey) != checkpoint {
		t.Fatal("stale writer overwrote the other page's checkpoint")
	}
}
