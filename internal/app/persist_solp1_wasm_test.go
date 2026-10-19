// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/store"
)

func solp1App(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	ds := store.SampleDataset()
	ds.Transactions = make([]domain.Transaction, 3300)
	for i := range ds.Transactions {
		ds.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("solp1-%04d", i), AccountID: ds.Accounts[0].ID,
			CategoryID: ds.Categories[0].ID, Payee: "Market", Desc: "Weekly shopping",
			Date: time.Now().UTC().Truncate(24 * time.Hour), Amount: money.New(-1200, "USD")}
	}
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old; a.Store().Close() })
	return a
}

func TestSOLP1RecategorisePersistenceWorkGuard(t *testing.T) {
	a := solp1App(t)
	oldHydrated, oldLocal, oldSuspended := hydrationSucceeded, hadLocalDataset, suspendAutosave
	oldQuarantined, oldPending := datasetQuarantined, quarantinePending
	oldEnvelope, oldPass, oldGen := pendingEnvelopeRaw, activePasscode, datasetMyGen
	oldHosted := currentHostedHydration()
	oldGeneration := readDatasetGen()
	oldSnap, oldUndoApp, oldUndoRev := lastUndoSnap, lastUndoCaptureApp, lastUndoCaptureRev
	oldApplying := applyingUndo
	oldStack, oldFeed := undoStack, auditview.Feed
	undoStack, auditview.Feed = history.NewStack(4*1024*1024), auditlog.New(500)
	hydrationSucceeded, hadLocalDataset, suspendAutosave = true, true, false
	datasetQuarantined, quarantinePending = false, false
	pendingEnvelopeRaw, activePasscode = "", ""
	initializeHostedHydration(false)
	t.Cleanup(func() {
		hydrationSucceeded, hadLocalDataset, suspendAutosave = oldHydrated, oldLocal, oldSuspended
		datasetQuarantined, quarantinePending = oldQuarantined, oldPending
		pendingEnvelopeRaw, activePasscode, datasetMyGen = oldEnvelope, oldPass, oldGen
		setHostedHydration(oldHosted)
		browserstore.SetMemory(datasetGenKey, oldGeneration)
		lastUndoSnap, lastUndoCaptureApp, lastUndoCaptureRev = oldSnap, oldUndoApp, oldUndoRev
		applyingUndo = oldApplying
		undoStack, auditview.Feed = oldStack, oldFeed
		persistPerfObserve = nil
	})
	initUndo()
	writes, exports, pushes := 0, 0, 0
	var complete func(error)
	var payload []byte
	var nextGeneration string
	save, force := newDatasetAutosave(a, func(values, expected map[string]string, done func(error)) {
		writes++
		payload = []byte(values[datasetStoreKey])
		nextGeneration = values[datasetGenKey]
		complete = done
	}, func(data []byte, _ time.Time) {
		pushes++
		if string(data) != string(payload) {
			t.Error("backend pushed a different snapshot from the committed one")
		}
	})
	persistPerfObserve = func(stage string) {
		if stage == "dataset-export" {
			exports++
		}
	}
	row := a.Transactions()[0]
	row.CategoryID = a.Categories()[1].ID
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	// Include the real undo/audit capture. Neither ticker nor explicit requests
	// may re-export while this revision's durable commit is outstanding.
	save()
	save()
	save()
	if writes != 1 || exports > 2 || pushes != 0 {
		t.Fatalf("recategorise: commits=%d exports=%d premature pushes=%d, want 1/<=2/0", writes, exports, pushes)
	}
	ds, err := store.Import(payload)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Transactions[0].CategoryID != row.CategoryID {
		t.Fatal("commit payload omitted recategorisation")
	}
	// A failure must retain dirty state and retry, without another full export.
	complete(errors.New("test durable commit rejected"))
	save()
	if writes != 2 || exports > 2 || pushes != 0 {
		t.Fatalf("retry: commits=%d exports=%d pushes=%d", writes, exports, pushes)
	}
	browserstore.SetMemory(datasetGenKey, nextGeneration)
	complete(nil)
	if pushes != 1 {
		t.Fatalf("durable commit produced %d backend pushes, want 1", pushes)
	}
	save()
	save()
	if writes != 2 || exports > 2 {
		t.Fatalf("idle repeats: commits=%d exports=%d, want 2/<=2", writes, exports)
	}
	t.Logf("SOLP1 guard: recategorise exports=%d commits=1; failure retries=1; idle exports=0 commits=0", exports)
	// Encryption-mode rewrites are distinct from ordinary persist requests, even
	// when a previous version is still committing and the dataset is unchanged.
	force()
	if writes != 3 {
		t.Fatalf("explicit rewrite made %d commits, want 3", writes)
	}
	force()
	browserstore.SetMemory(datasetGenKey, nextGeneration)
	complete(nil)
	save()
	if writes != 4 || exports > 2 {
		t.Fatalf("rewrite requested during commit was lost: commits=%d exports=%d", writes, exports)
	}
	browserstore.SetMemory(datasetGenKey, nextGeneration)
	complete(nil)
}

func TestSOLP1PersistProfile3300(t *testing.T) {
	a := solp1App(t)
	measure := func(label string, f func()) {
		start := time.Now()
		f()
		t.Logf("SOLP1-PERF %s %.3f ms", label, float64(time.Since(start).Microseconds())/1000)
	}
	measure("dataset export", func() {
		if _, err := localDatasetExport(a); err != nil {
			t.Fatal(err)
		}
	})
	measure("undo snapshot export+decode", func() {
		if _, err := currentSnapshot(); err != nil {
			t.Fatal(err)
		}
	})
	measure("notification candidates", func() { _ = buildNotifyCandidates(a, time.Now(), notify.RuleConfig{}) })
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	data, err := localDatasetExport(a)
	if err != nil {
		t.Fatal(err)
	}
	measure("ENC3 SealRecord+envelope encoding", func() {
		env, err := atrest.SealRecord(wk, keyring.LocalScope(""), "solp1-fixture", datasetStoreKey, data)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Encode(); err != nil {
			t.Fatal(err)
		}
	})
}
