// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"testing"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

// TestPRODREVIEWLegacyPlaintextSnapshotWithAppLockCanReachMigration records
// the production migration case: existing plaintext must be opened locally,
// then saved under App Lock before hosted hydration releases the gate.
func TestPRODREVIEWLegacyPlaintextSnapshotWithAppLockCanReachMigration(t *testing.T) {
	fixpullFixture(t)
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
		out.Workspace.UpdatedAt = "2026-10-19T12:00:00Z"
		out.Dataset = []byte(`{"schemaVersion":2,"accounts":[]}`)
		return nil
	}

	pullActiveWorkspaceFromBackend(false)
	fixpullWait(t, func() bool {
		return currentHostedHydration().Phase == hostedHydrationReady || currentHostedHydration().Phase == hostedHydrationError
	})
	if got := currentHostedHydration(); got.Phase == hostedHydrationError {
		t.Fatalf("legacy snapshot left the hosted gate in an error instead of migrating locally: %q", got.Message)
	}
}
