// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// The focused lane does not run ENC31's process-wide setup. Initialize the same
// isolated Node backend only if no earlier test has opened protected storage.
func solp22Storage(t *testing.T) {
	t.Helper()
	if browserstore.SecureKeyState() == nil {
		if !js.Global().Get("process").Truthy() {
			t.Fatal("SOLP22 requires the isolated Node IndexedDB fixture")
		}
		js.Global().Set("crypto", js.Global().Get("require").Invoke("node:crypto").Get("webcrypto"))
		js.Global().Set("BroadcastChannel", js.Undefined())
		js.Global().Set("localStorage", js.Undefined())
		script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
		if err != nil {
			t.Fatal(err)
		}
		js.Global().Call("eval", string(script))
		browserstore.Init()
	}
	if err := browserstore.InitError(); err != nil {
		t.Fatal(err)
	}
	if !browserstore.Durable() {
		t.Fatal("fixture has no strict durable backend")
	}
	// Other incident tests intentionally leave this page's cache stale. A new
	// fixture starts from durable scope/generation metadata, just as a reload
	// would, rather than granting its writer an obsolete cached generation.
	browserstore.Barrier()
	browserstore.Reload(datasetGenKey, datasetHeadKey, datasetOwnerKey, workspacesKey)
	boot2FixtureRanges(t)
}

func solp22Batch(t *testing.T, values, expected map[string]string) {
	t.Helper()
	if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecureBatch(values, expected, done) }); err != nil {
		t.Fatal(err)
	}
}

type solp22Fixture struct {
	app        *appstate.App
	row        domain.Transaction
	checkpoint string
	atom       state.Atom[prefs.Prefs]
}

// Each fixture commits an unchanged checkpoint, then edits exactly one row via
// the production record writer. Reload assertions read protected persisted bytes.
func solp22Journal(t *testing.T) solp22Fixture {
	t.Helper()
	return solp22JournalDataset(t, store.SampleDataset(), "solp22-workspace")
}

func solp22JournalDataset(t *testing.T, ds store.Dataset, workspaceID string) solp22Fixture {
	t.Helper()
	solp22Storage(t)
	browserstore.Barrier()
	initialJournal := map[string]bool{}
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
			initialJournal[key] = true
		}
	}
	f := render.New(t)
	oldApp, oldPasscode, oldPending := appstate.Default, activePasscode, pendingEnvelopeRaw
	oldHydrated, oldLocal, oldSuspend := hydrationSucceeded, hadLocalDataset, suspendAutosave
	oldQ, oldQP, oldRaw, oldAvailable := datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable
	oldHosted, oldGen := currentHostedHydration(), datasetMyGen
	oldHApp, oldHCursor, oldHRev := hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision
	oldUndoPending, oldUndoCursor := undoPending, undoCaptureCursor
	oldUser := browserstore.GetString(syncUserIDKey)
	keys := []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey,
		appLockKey, credVaultStoreKey, aiCredentialStoreKey, webauthnCredKey, webauthnSaltKey,
		webauthnVaultKey, secureLockTransitionKey, syncMetaKey(workspaceID)}
	previous := map[string]string{}
	for _, key := range keys {
		previous[key] = browserstore.GetString(key)
	}
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	appstate.Default = a
	activePasscode, pendingEnvelopeRaw = "", ""
	hydrationSucceeded, hadLocalDataset, suspendAutosave = true, true, false
	datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = false, false, "", false
	initializeHostedHydration(false)
	undoPending, undoCaptureCursor = nil, 0
	t.Cleanup(func() {
		a.SetDurableWriter(nil)
		browserstore.Barrier()
		if browserstore.SecureKeyState().AppLock {
			if err := enc31Wait(t, browserstore.DisableAppLockSecure); err != nil {
				t.Error(err)
			}
		}
		solp22Batch(t, previous, nil)
		// Assertions have finished and original metadata is durable again. Remove
		// only this fixture's detached records from the isolated Node backend;
		// later snapshot/wipe tests must not inherit encrypted fixture history.
		var detachedRecords []string
		for _, key := range browserstore.Keys() {
			if !initialJournal[key] && strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
				detachedRecords = append(detachedRecords, key)
			}
		}
		jrnlRemoveFixtureRecords(t, detachedRecords)
		browserstore.SetMemory(syncUserIDKey, oldUser)
		f.Cleanup()
		appstate.Default, activePasscode, pendingEnvelopeRaw = oldApp, oldPasscode, oldPending
		hydrationSucceeded, hadLocalDataset, suspendAutosave = oldHydrated, oldLocal, oldSuspend
		datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = oldQ, oldQP, oldRaw, oldAvailable
		setHostedHydration(oldHosted)
		datasetMyGen = oldGen
		hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = oldHApp, oldHCursor, oldHRev
		undoPending, undoCaptureCursor = oldUndoPending, oldUndoCursor
		a.Store().Close()
	})
	ds.Transactions = ds.Transactions[:2]
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	uistate.PersistPrefs(prefs.Default())
	var atom state.Atom[prefs.Prefs]
	f.Render(ui.CreateElement(func() ui.Node {
		atom = state.UseAtom("solp22-preferences", prefs.Default())
		return nil
	}))
	f.Stabilize()
	checkpoint, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := json.Marshal(workspace.Registry{}.Add(workspaceID, "Journal test"))
	values := map[string]string{}
	for _, key := range keys {
		values[key] = ""
	}
	values[datasetStoreKey] = encodeRecordCheckpoint(checkpoint)
	values[datasetGenKey], values[workspacesKey] = id.New(), string(registry)
	solp22Batch(t, values, nil)
	datasetMyGen = values[datasetGenKey]
	hydratedRecordApp, hydratedRecordRevision = a, a.Rev()
	hydratedRecordCursor, err = a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	newDatasetAutosave(a, browserstore.SetSecureBatch, nil)
	row := a.Transactions()[0]
	row.Desc = "SOLP22 durable journal edit"
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) {
		a.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
	}); err != nil {
		t.Fatal(err)
	}
	if browserstore.GetString(datasetStoreKey) != values[datasetStoreKey] || browserstore.GetString(datasetHeadKey) == "" {
		t.Fatal("fixture did not persist a journal-only edit")
	}
	return solp22Fixture{a, row, values[datasetStoreKey], atom}
}

func solp22AssertRow(t *testing.T, portable string, row domain.Transaction) store.Dataset {
	t.Helper()
	ds, err := store.Import([]byte(portable))
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range ds.Transactions {
		if got.ID == row.ID && got.Desc == row.Desc {
			return ds
		}
	}
	t.Fatal("reloaded portable dataset lost the journaled row edit")
	return ds
}

func solp22Reload(t *testing.T, row domain.Transaction) store.Dataset {
	t.Helper()
	raw, _, err := materializeLocalDataset()
	if err != nil {
		t.Fatal(err)
	}
	return solp22AssertRow(t, raw, row)
}

func TestSOLP22JournalReplacementPaths(t *testing.T) {
	t.Run("account replacement stash and restore", func(t *testing.T) {
		fixture := solp22Journal(t)
		owner := syncstate.DatasetOwner{ServerID: "solp22-previous-server", UserID: "solp22-previous-user"}
		encoded, _ := json.Marshal(owner)
		key := foreignDatasetStashKey(owner)
		oldStash := browserstore.GetString(key)
		t.Cleanup(func() { solp22Batch(t, map[string]string{key: oldStash}, nil) })
		solp22Batch(t, map[string]string{datasetOwnerKey: string(encoded)}, nil)
		browserstore.SetMemory(syncUserIDKey, "solp22-arriving-user")
		// The live store is deliberately changed already: a stash must come from
		// the prior owner's persisted chain, never from a new account's live rows.
		newRow := fixture.row
		newRow.Desc = "Arriving account's in-memory row"
		if err := fixture.app.PutTransaction(newRow); err != nil {
			t.Fatal(err)
		}
		if err := stashForeignDataset(); err != nil {
			t.Fatal(err)
		}
		stash := browserstore.GetString(key)
		solp22AssertRow(t, stash, fixture.row)
		if strings.Contains(stash, "cashfluxRecordCheckpoint") {
			t.Fatal("stash is not a portable dataset")
		}
		empty, _ := store.Export(store.EmptyDataset())
		if err := enc31Wait(t, func(done func(error)) {
			applyBundleThen(map[string]string{datasetStoreKey: string(empty)}, func() { done(nil) })
		}); err != nil {
			t.Fatal(err)
		}
		// No production stash reader exists yet. Exercise the existing restore
		// boundary with the portable recovery copy a future restore UI will read.
		if err := enc31Wait(t, func(done func(error)) {
			applyBundleThen(map[string]string{datasetStoreKey: stash}, func() { done(nil) })
		}); err != nil {
			t.Fatal(err)
		}
		solp22Reload(t, fixture.row)
	})
	t.Run("cloud preferences and reload", func(t *testing.T) {
		fixture := solp22Journal(t)
		before := readDatasetGen()
		next := prefs.Default()
		next.WeekStart = prefs.WeekMonday
		if err := enc31Wait(t, func(done func(error)) { cloudCommitPreferences(fixture.atom, next, done) }); err != nil {
			t.Fatal(err)
		}
		if browserstore.GetString(datasetHeadKey) != "" || datasetMyGen == before || readDatasetGen() != datasetMyGen {
			t.Fatal("cloud replacement did not atomically supersede the journal")
		}
		ds := solp22Reload(t, fixture.row)
		var got prefs.Prefs
		if err := json.Unmarshal([]byte(ds.SettingsKV["cashflux:prefs"]), &got); err != nil || got.WeekStart != next.WeekStart {
			t.Fatalf("reloaded cloud preferences = %+v, %v", got, err)
		}
	})
	t.Run("stash failure preserves the prior account", func(t *testing.T) {
		fixture := solp22Journal(t)
		owner := syncstate.DatasetOwner{ServerID: "solp22-failure-server", UserID: "solp22-prior-user"}
		encoded, _ := json.Marshal(owner)
		key := foreignDatasetStashKey(owner)
		previous := browserstore.GetString(key)
		t.Cleanup(func() { solp22Batch(t, map[string]string{key: previous}, nil) })
		solp22Batch(t, map[string]string{datasetOwnerKey: string(encoded), key: "older recovery copy"}, nil)
		browserstore.SetMemory(syncUserIDKey, "solp22-next-user")
		head, generation := browserstore.GetString(datasetHeadKey), readDatasetGen()
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		if err := stashForeignDataset(); err == nil {
			t.Fatal("failed durable stash acknowledged")
		}
		if browserstore.GetString(key) != "older recovery copy" || browserstore.GetString(datasetHeadKey) != head || readDatasetGen() != generation || browserstore.GetString(datasetOwnerKey) != string(encoded) {
			t.Fatal("failed stash changed the prior owner's durable data")
		}
		solp22Reload(t, fixture.row)
	})
	t.Run("damaged journal blocks stash and exposes recovery", func(t *testing.T) {
		fixture := solp22Journal(t)
		owner := syncstate.DatasetOwner{ServerID: "solp22-damaged-server", UserID: "solp22-prior-user"}
		encoded, _ := json.Marshal(owner)
		key := foreignDatasetStashKey(owner)
		oldStash := browserstore.GetString(key)
		beforeKeys := map[string]bool{}
		for _, key := range browserstore.Keys() {
			beforeKeys[key] = true
		}
		t.Cleanup(func() {
			solp22Batch(t, map[string]string{key: oldStash}, nil)
			for _, key := range browserstore.Keys() {
				if strings.HasPrefix(key, quarantinePrefix) && !beforeKeys[key] {
					if err := enc31Wait(t, func(done func(error)) { browserstore.RemoveSecure(key, done) }); err != nil {
						t.Error(err)
					}
				}
			}
		})
		var head datasetRecordHead
		if err := json.Unmarshal([]byte(browserstore.GetString(datasetHeadKey)), &head); err != nil {
			t.Fatal(err)
		}
		head.Tail = recordWorkspacePrefix(recordWorkspace()) + "missing-batch"
		broken, _ := json.Marshal(head)
		solp22Batch(t, map[string]string{datasetHeadKey: string(broken), datasetOwnerKey: string(encoded), key: "older recovery copy"}, nil)
		browserstore.SetMemory(syncUserIDKey, "solp22-next-user")
		if err := stashForeignDataset(); err == nil {
			t.Fatal("damaged journal was stashed as an older checkpoint")
		}
		browserstore.Barrier()
		if !datasetQuarantined || !quarantineAvailable || !autosaveBlocked() || browserstore.GetString(key) != "older recovery copy" || browserstore.GetString(datasetStoreKey) != fixture.checkpoint || browserstore.GetString(datasetHeadKey) != string(broken) {
			t.Fatal("damaged prior account was replaced or recovery is unavailable")
		}
	})
	t.Run("cloud failure retains checkpoint head and entitlement", func(t *testing.T) {
		fixture := solp22Journal(t)
		head, generation := browserstore.GetString(datasetHeadKey), readDatasetGen()
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		next := prefs.Default()
		next.WeekStart = prefs.WeekMonday
		err := enc31Wait(t, func(done func(error)) { cloudCommitPreferences(fixture.atom, next, done) })
		if err == nil || suspendAutosave || datasetMyGen != generation || readDatasetGen() != generation || browserstore.GetString(datasetHeadKey) != head || browserstore.GetString(datasetStoreKey) != fixture.checkpoint {
			t.Fatalf("failed cloud write changed durable state: %v", err)
		}
		solp22Reload(t, fixture.row)
	})
	t.Run("App Lock prepare and interrupted recovery", func(t *testing.T) {
		fixture := solp22Journal(t)
		config := applock.Config{}.WithPasscode("solp22-new-passcode", "solp22-salt", 0, "")
		transition, err := prepareSecureLockTransition("solp22-new-passcode", config, true)
		if err != nil {
			t.Fatal(err)
		}
		solp22AssertRow(t, transition.Values[datasetStoreKey], fixture.row)
		if transition.Values[datasetHeadKey] != "" || transition.Expected[datasetGenKey] != readDatasetGen() {
			t.Fatal("App Lock replacement omitted journal CAS")
		}
		raw, _ := json.Marshal(transition)
		solp22Batch(t, map[string]string{secureLockTransitionKey: string(raw)}, nil)
		passcode, changed, err := recoverSecureLockTransition()
		if err != nil || !changed || passcode != "solp22-new-passcode" {
			t.Fatalf("recover transition = %v, %v", changed, err)
		}
		if browserstore.GetString(datasetHeadKey) != "" || datasetMyGen != readDatasetGen() {
			t.Fatal("recovered transition did not adopt its replacement")
		}
		solp22Reload(t, fixture.row)
		if err := runSecureLockTransition("", applock.Config{}, true); err != nil {
			t.Fatal(err)
		}
		solp22Reload(t, fixture.row)
	})
	t.Run("App Lock rejects a newer journal generation", func(t *testing.T) {
		fixture := solp22Journal(t)
		transition, err := prepareSecureLockTransition("", applock.Config{}, true)
		if err != nil {
			t.Fatal(err)
		}
		row := fixture.row
		row.Desc = "Newer durable journal edit"
		if err := fixture.app.PutTransaction(row); err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) {
			fixture.app.RequestDurable(func(result appstate.DurableResult) { done(result.Err) })
		}); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(transition)
		solp22Batch(t, map[string]string{secureLockTransitionKey: string(raw)}, nil)
		if err := commitSecureLockTransition(transition, string(raw)); !errors.Is(err, atrest.ErrRecordChanged) {
			t.Fatalf("stale App Lock transition = %v", err)
		}
		solp22Reload(t, row)
	})
	t.Run("synced replacement metadata and failure", func(t *testing.T) {
		fixture := solp22Journal(t)
		portable, _, err := materializeLocalDataset()
		if err != nil {
			t.Fatal(err)
		}
		meta := syncMeta{Version: 42, Owner: "solp22-arriving-user"}
		head, generation := browserstore.GetString(datasetHeadKey), readDatasetGen()
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		completed := false
		saveSyncedDatasetThen("solp22-workspace", []byte(portable), meta, func() { completed = true })
		browserstore.Barrier()
		if completed || browserstore.GetString(datasetHeadKey) != head || datasetMyGen != generation || browserstore.GetString(syncMetaKey("solp22-workspace")) != "" {
			t.Fatal("failed sync replacement acknowledged or partly committed")
		}
		if err := enc31Wait(t, func(done func(error)) {
			saveSyncedDatasetThen("solp22-workspace", []byte(portable), meta, func() { done(nil) })
		}); err != nil {
			t.Fatal(err)
		}
		if browserstore.GetString(datasetHeadKey) != "" || datasetMyGen == generation || browserstore.GetString(syncMetaKey("solp22-workspace")) == "" {
			t.Fatal("synced replacement did not commit metadata and generation")
		}
		solp22Reload(t, fixture.row)
	})
}
