// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/base64"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

func fixpullFixture(t *testing.T) {
	t.Helper()
	solp22Storage(t)
	const workspaceID = "fixpull-workspace"
	pr := prefs.Default()
	pr.ServerURL, pr.ServerToken, pr.BackendDisabled = "https://fixpull.test", "fixpull-token", false
	marker := encryptedWorkspaceKeyFor(datasetServerIDForURL(pr.ServerURL), "fixpull-account", workspaceID)
	keys := []string{
		datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey,
		workspacesKey, syncUserIDKey, syncStatusKey, syncQueueKey,
		syncMetaPrefix + workspaceID, marker, appLockKey, secureLockTransitionKey,
	}
	previous := make(map[string]string, len(keys))
	for _, key := range keys {
		previous[key] = browserstore.GetString(key)
	}
	initialJournal := make(map[string]bool)
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
			initialJournal[key] = true
		}
	}
	oldPrefs, oldPasscode, oldSample := uistate.LoadPrefs(), activePasscode, uistate.SampleActive()
	oldHosted := currentHostedHydration()
	oldHadLocal, oldMerged := hadLocalDataset, mergedRemoteWorkspaces
	oldDatasetMyGen := datasetMyGen
	pageRefreshState.Lock()
	oldScope, oldReady, oldBlocked := pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = nil, true, false
	pageRefreshState.Unlock()
	blank := make(map[string]string, len(previous))
	for key := range previous {
		blank[key] = ""
	}
	solp22Batch(t, blank, nil)
	datasetMyGen = ""
	uistate.PersistPrefs(pr)
	saveRegistry(workspace.Registry{}.Add(workspaceID, "Household"))
	lsSet(syncUserIDKey, "fixpull-account")
	saveAppLock(applock.Config{})
	activePasscode = ""
	uistate.SetSampleActive(false)
	hadLocalDataset, mergedRemoteWorkspaces = true, false
	initializeHostedHydration(true)
	t.Cleanup(func() {
		browserstore.Barrier()
		solp22Batch(t, previous, nil)
		var detachedRecords []string
		for _, key := range browserstore.Keys() {
			if !initialJournal[key] && strings.HasPrefix(key, recordWorkspacePrefix(workspaceID)) {
				detachedRecords = append(detachedRecords, key)
			}
		}
		jrnlRemoveFixtureRecords(t, detachedRecords)
		uistate.PersistPrefs(oldPrefs)
		activePasscode = oldPasscode
		uistate.SetSampleActive(oldSample)
		hadLocalDataset, mergedRemoteWorkspaces = oldHadLocal, oldMerged
		datasetMyGen = oldDatasetMyGen
		setHostedHydration(oldHosted)
		pageRefreshState.Lock()
		pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = oldScope, oldReady, oldBlocked
		pageRefreshState.Unlock()
	})
}

func fixpullImportApp(t *testing.T) *appstate.App {
	t.Helper()
	oldApp := appstate.Default
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	appstate.Default = app
	t.Cleanup(func() {
		appstate.Default = oldApp
		app.Store().Close()
	})
	return app
}

func fixpullWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("workspace pull did not reach the expected state")
}

func TestFIXPULLHostedDeviceWithoutAppLockPullsEncryptedSnapshot(t *testing.T) {
	fixpullFixture(t)
	oldRPC := hostedPullWorkspaceRPC
	t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
	calls := 0
	envelope := cryptobox.Marshal(cryptobox.Envelope{
		V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM,
		Salt:   base64.StdEncoding.EncodeToString([]byte("salt")),
		IV:     base64.StdEncoding.EncodeToString([]byte("iv")),
		Cipher: base64.StdEncoding.EncodeToString([]byte("cipher")),
	})
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		calls++
		if req.ID != "fixpull-workspace" {
			t.Errorf("GetWorkspace workspace = %q", req.ID)
		}
		out.Found = true
		out.Workspace.ID = req.ID
		out.Dataset = envelope
		return nil
	}
	pullActiveWorkspaceFromBackend(false)
	fixpullWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationLocked })
	if calls != 1 {
		t.Fatalf("GetWorkspace calls = %d, want one", calls)
	}
	if got := currentHostedHydration(); !got.Required || len(got.EncryptedSnapshot) == 0 {
		t.Fatalf("hosted hydration state = %#v, want locked with retained encrypted snapshot", got)
	}
}

// A device without App Lock may hold an encrypted snapshot behind the lock form,
// but it must never import a plaintext server copy (encryption downgrade).
// Accounts that synced before encryption was required hold a legacy plaintext
// snapshot. A device without App Lock must be able to open it (refusing it locked
// the owner out in 1.19.3); a device with App Lock still refuses any plaintext.
func TestFIXPULLLegacyPlaintextLoadsWithoutAppLockOnly(t *testing.T) {
	dataset := store.SampleDataset()
	dataset.Transactions, dataset.Artifacts = nil, nil
	plaintext, err := store.Export(dataset)
	if err != nil {
		t.Fatal(err)
	}
	serve := func() {
		hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
			out.Found = true
			out.Workspace.ID = req.ID
			out.Workspace.Version = 3
			out.Workspace.UpdatedAt = "2026-10-20T12:00:00Z"
			out.Dataset = plaintext
			return nil
		}
	}
	const refused = "The server returned an unprotected copy. CashFlux did not load it."

	t.Run("without App Lock the legacy copy is not refused", func(t *testing.T) {
		fixpullFixture(t)
		app := fixpullImportApp(t)
		hadLocalDataset = false
		if loadAppLock().Active() || serverWorkspaceWasEncrypted(activeWorkspaceID()) {
			t.Fatal("legacy fixture retained App Lock or encrypted-server state")
		}
		oldRPC := hostedPullWorkspaceRPC
		t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
		serve()
		pullActiveWorkspaceFromBackend(false)
		fixpullWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationProtect })
		if got := currentHostedHydration(); got.Message == refused || !legacyWorkspaceNeedsProtection() {
			t.Fatalf("legacy plaintext did not import and request protection: hydration=%#v legacy=%t", got, legacyWorkspaceNeedsProtection())
		}
		meta := loadSyncMeta(activeWorkspaceID())
		if !meta.LegacyPlaintext || meta.Version != 3 || meta.Server != currentDatasetServerID() {
			t.Fatalf("legacy sync metadata = %#v, want imported version 3 marked for protection", meta)
		}
		got, err := app.Store().Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Accounts) != len(dataset.Accounts) {
			t.Fatalf("imported accounts = %d, want %d", len(got.Accounts), len(dataset.Accounts))
		}
		wantIDs := make(map[string]bool, len(dataset.Accounts))
		for _, account := range dataset.Accounts {
			wantIDs[account.ID] = true
		}
		for _, account := range got.Accounts {
			if !wantIDs[account.ID] {
				t.Fatalf("imported unexpected account %q", account.ID)
			}
			delete(wantIDs, account.ID)
		}
		if len(wantIDs) != 0 {
			t.Fatalf("import omitted account IDs: %v", wantIDs)
		}
	})
	t.Run("with App Lock plaintext is refused", func(t *testing.T) {
		fixpullFixture(t)
		oldRPC, oldPass := hostedPullWorkspaceRPC, activePasscode
		t.Cleanup(func() { hostedPullWorkspaceRPC, activePasscode = oldRPC, oldPass })
		activePasscode = "known-app-lock"
		saveAppLock(applock.Config{}.WithPasscode(activePasscode, "fixpull-test-salt", 0, ""))
		if !datasetEncryptionActive() {
			t.Fatal("fixture did not activate dataset encryption with persisted App Lock")
		}
		serve()
		pullActiveWorkspaceFromBackend(false)
		fixpullWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationError })
		if got := currentHostedHydration(); got.Message != refused {
			t.Fatalf("hosted hydration state = %#v, want the unprotected-copy refusal", got)
		}
	})
	t.Run("without App Lock a previously encrypted scope rejects plaintext", func(t *testing.T) {
		fixpullFixture(t)
		marker := encryptedWorkspaceKey(activeWorkspaceID())
		lsSet(marker, "true")
		if loadAppLock().Active() {
			t.Fatal("downgrade fixture unexpectedly has App Lock enabled")
		}
		oldRPC := hostedPullWorkspaceRPC
		t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
		serve()
		pullActiveWorkspaceFromBackend(false)
		fixpullWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationError })
		if got := currentHostedHydration(); got.Message != refused {
			t.Fatalf("hosted hydration state = %#v, want the unprotected-copy refusal", got)
		}
	})
}

func TestFIXPULLAppLockKeepsExistingEncryptedPullHandling(t *testing.T) {
	fixpullFixture(t)
	activePasscode = "known-app-lock"
	saveAppLock(applock.Config{}.WithPasscode(activePasscode, "fixpull-test-salt", 0, ""))
	if !datasetEncryptionActive() {
		t.Fatal("fixture did not activate encrypted dataset handling with persisted App Lock")
	}
	oldRPC := hostedPullWorkspaceRPC
	t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
	envelope := cryptobox.Marshal(cryptobox.Envelope{
		V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM, Salt: "!", IV: "!", Cipher: "!",
	})
	calls := 0
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		calls++
		out.Found, out.Workspace.ID, out.Dataset = true, req.ID, envelope
		return nil
	}
	pullActiveWorkspaceFromBackend(false)
	fixpullWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationLocked })
	if calls != 1 {
		t.Fatalf("GetWorkspace calls = %d, want one with App Lock active", calls)
	}
	if got := loadSyncStatus().State; got != syncStateLockedDifferentPasscode {
		t.Fatalf("sync status = %q, want existing different-passcode handling", got)
	}
}

func TestFIXPULLPushGuardStillRejectsUnencryptedDataset(t *testing.T) {
	fixpullFixture(t)
	saveSyncQueue([]queuedSyncMutation{{
		WorkspaceID: "fixpull-workspace", Dataset: `{"secret":"plaintext"}`, Hash: "fixpull-plaintext",
	}})
	flushBackendSyncQueue()
	status := loadSyncStatus()
	if status.State != string(syncstate.StatusNeedsDataProtection) {
		t.Fatalf("sync status = %q, want data-protection-required", status.State)
	}
	queue := loadSyncQueue()
	if len(queue) != 1 || queue[0].Dataset != `{"secret":"plaintext"}` {
		t.Fatalf("guard changed or dropped the queued snapshot: %#v", queue)
	}
}
