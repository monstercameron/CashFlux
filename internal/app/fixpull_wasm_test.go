// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

func fixpullFixture(t *testing.T) {
	t.Helper()
	keys := []string{workspacesKey, syncUserIDKey, syncStatusKey, syncQueueKey, syncMetaPrefix + "fixpull-workspace"}
	previous := make(map[string]string, len(keys))
	for _, key := range keys {
		previous[key] = lsGet(key)
	}
	oldPrefs, oldPasscode, oldSample := uistate.LoadPrefs(), activePasscode, uistate.SampleActive()
	oldHosted := currentHostedHydration()
	oldHadLocal, oldMerged := hadLocalDataset, mergedRemoteWorkspaces
	pageRefreshState.Lock()
	oldScope, oldReady, oldBlocked := pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = nil, true, false
	pageRefreshState.Unlock()
	pr := prefs.Default()
	pr.ServerURL, pr.ServerToken, pr.BackendDisabled = "https://fixpull.test", "fixpull-token", false
	uistate.PersistPrefs(pr)
	saveRegistry(workspace.Registry{}.Add("fixpull-workspace", "Household"))
	lsSet(syncUserIDKey, "fixpull-account")
	lsRemove(syncStatusKey)
	lsRemove(syncQueueKey)
	activePasscode = ""
	uistate.SetSampleActive(false)
	hadLocalDataset, mergedRemoteWorkspaces = true, false
	initializeHostedHydration(true)
	t.Cleanup(func() {
		for key, value := range previous {
			if value == "" {
				lsRemove(key)
			} else {
				lsSet(key, value)
			}
		}
		uistate.PersistPrefs(oldPrefs)
		activePasscode = oldPasscode
		uistate.SetSampleActive(oldSample)
		hadLocalDataset, mergedRemoteWorkspaces = oldHadLocal, oldMerged
		setHostedHydration(oldHosted)
		pageRefreshState.Lock()
		pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = oldScope, oldReady, oldBlocked
		pageRefreshState.Unlock()
	})
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
	plaintext := []byte(`{"schemaVersion":2,"accounts":[]}`)
	serve := func() {
		hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
			out.Found = true
			out.Workspace.ID = req.ID
			out.Dataset = plaintext
			return nil
		}
	}
	const refused = "The server returned an unprotected copy. CashFlux did not load it."

	t.Run("without App Lock the legacy copy is not refused", func(t *testing.T) {
		fixpullFixture(t)
		oldRPC := hostedPullWorkspaceRPC
		t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
		serve()
		pullActiveWorkspaceFromBackend(false)
		fixpullWait(t, func() bool { return currentHostedHydration().Phase != hostedHydrationLoading })
		if got := currentHostedHydration(); got.Message == refused {
			t.Fatalf("legacy plaintext was refused without App Lock: %#v", got)
		}
	})
	t.Run("with App Lock plaintext is refused", func(t *testing.T) {
		fixpullFixture(t)
		oldRPC, oldPass := hostedPullWorkspaceRPC, activePasscode
		t.Cleanup(func() { hostedPullWorkspaceRPC, activePasscode = oldRPC, oldPass })
		activePasscode = "known-app-lock"
		if !datasetEncryptionActive() {
			t.Skip("fixture cannot activate dataset encryption via activePasscode")
		}
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

func TestHostedFirstOpenHoldsUploadsUntilAccountDataIsResolved(t *testing.T) {
	fixpullFixture(t)
	oldLock, oldPasscode := loadAppLock(), activePasscode
	t.Cleanup(func() {
		saveAppLock(oldLock)
		activePasscode = oldPasscode
	})
	activePasscode = "hosted-recovery-test-passcode"
	saveAppLock(applock.Config{}.WithPasscode(activePasscode, "hosted-recovery-test-salt", 0, ""))
	if !datasetEncryptionActive() {
		t.Fatal("test needs an active App Lock to reach the queue flush")
	}
	if reason := pushBlockedReason(); reason != "account data must be loaded from the server first" {
		t.Fatalf("new snapshot would be queued before account recovery: %q", reason)
	}
	setSyncStatus(syncStatus{State: "queued", Pending: 1})
	flushBackendSyncQueue()
	time.Sleep(100 * time.Millisecond)
	if status := loadSyncStatus(); status.State != "queued" {
		t.Fatalf("queue flush ran before the server pull: %+v", status)
	}
	if !hostedHydrationRequired() {
		t.Fatal("queue guard released the hosted recovery gate")
	}
}
