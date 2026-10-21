// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// TestPRODREVIEWAppLockRejectsLegacyPlaintextSnapshotBeforeImport records that
// App Lock rejects an unprotected hosted snapshot before it can be imported.
func TestPRODREVIEWAppLockRejectsLegacyPlaintextSnapshotBeforeImport(t *testing.T) {
	fixpullFixture(t)
	app := fixpullImportApp(t)
	// Pulling may persist the already selected local connection preferences.
	// Include those local preferences in the baseline before checking that no
	// server financial data was imported.
	uistate.PersistPrefs(uistate.LoadPrefs())
	before, err := app.Store().Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	dataset := store.SampleDataset()
	if len(dataset.Accounts) == 0 {
		t.Fatal("sample dataset has no account for the plaintext-import sentinel")
	}
	dataset.Accounts[0].Name = "PRODREVIEW plaintext must not import"
	plaintext, err := store.Export(dataset)
	if err != nil {
		t.Fatal(err)
	}

	oldRPC, oldPasscode, oldLock := hostedPullWorkspaceRPC, activePasscode, lsGet(appLockKey)
	t.Cleanup(func() {
		hostedPullWorkspaceRPC, activePasscode = oldRPC, oldPasscode
		if oldLock == "" {
			lsRemove(appLockKey)
		} else {
			lsSet(appLockKey, oldLock)
		}
	})
	activePasscode = "prodreview-known-passcode"
	saveAppLock(applock.Config{}.WithPasscode(activePasscode, "prodreview-salt", 0, ""))
	if !datasetEncryptionActive() {
		t.Fatal("fixture did not activate App Lock encryption")
	}
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		out.Found = true
		out.Workspace.ID = req.ID
		out.Workspace.Version = 3
		out.Workspace.UpdatedAt = "2026-10-19T12:00:00Z"
		out.Dataset = plaintext
		return nil
	}

	pullActiveWorkspaceFromBackend(false)
	fixpullWait(t, func() bool {
		return currentHostedHydration().Phase == hostedHydrationReady || currentHostedHydration().Phase == hostedHydrationError
	})
	if got := currentHostedHydration(); got.Phase != hostedHydrationError || got.Message != "The server returned an unprotected copy. CashFlux did not load it." {
		t.Fatalf("hosted hydration = %#v, want plaintext rejection before import", got)
	}
	after, err := app.Store().Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("plaintext snapshot changed the local dataset: before=%#v after=%#v", before, after)
	}
}
