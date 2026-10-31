// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
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
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func migrationFixture(t *testing.T) []byte {
	t.Helper()
	solp22Storage(t)
	oldApp, oldGen := appstate.Default, datasetMyGen
	oldHydrated, oldSuspend := hydrationSucceeded, suspendAutosave
	previousRecords := map[string]string{}
	for _, key := range []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, appLockKey, credVaultStoreKey, aiCredentialStoreKey, secureLockTransitionKey} {
		previousRecords[key] = browserstore.GetString(key)
	}
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	appstate.Default = app
	blank := map[string]string{}
	for key := range previousRecords {
		blank[key] = ""
	}
	solp22Batch(t, blank, nil)
	datasetMyGen = ""
	hydrationSucceeded, suspendAutosave = true, false
	t.Cleanup(func() {
		browserstore.Barrier()
		if browserstore.SecureKeyState().AppLock {
			if err := enc31Wait(t, browserstore.DisableAppLockSecure); err != nil {
				t.Error(err)
			}
		}
		solp22Batch(t, previousRecords, nil)
		appstate.Default, datasetMyGen = oldApp, oldGen
		hydrationSucceeded, suspendAutosave = oldHydrated, oldSuspend
		app.Store().Close()
	})
	fixpullFixture(t)
	hadLocalDataset = false
	oldPull, oldPush, oldResave := hostedPullWorkspaceRPC, hostedPushWorkspaceRPC, resaveDataset
	resaveDataset = nil
	marker := encryptedWorkspaceKey(activeWorkspaceID())
	previous := browserstore.GetString(marker)
	lsRemove(marker)
	t.Cleanup(func() {
		browserstore.Barrier()
		hostedPullWorkspaceRPC, hostedPushWorkspaceRPC, resaveDataset = oldPull, oldPush, oldResave
		solp22Batch(t, map[string]string{marker: previous}, nil)
	})
	saveAppLock(applock.Config{})
	ds := store.SampleDataset()
	ds.Transactions, ds.Artifacts = ds.Transactions[:2], nil
	data, err := store.Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		*out = backendrpc.GetWorkspaceResponse{Found: true, Workspace: backendrpc.Workspace{ID: req.ID, Version: 7, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, Dataset: data}
		return nil
	}
	return data
}

func migrationWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("migration did not finish: hydration=%+v sync=%+v", currentHostedHydration(), loadSyncStatus())
}

func TestLegacyMigrationLoadsPromptsEncryptsAndRefusesDowngrade(t *testing.T) {
	data := migrationFixture(t)
	pushes := make(chan backendrpc.PutWorkspaceRequest, 2)
	hostedPushWorkspaceRPC = func(_ context.Context, pr prefs.Prefs, req backendrpc.PutWorkspaceRequest, out *backendrpc.PutWorkspaceResponse) error {
		if pr.ServerURL != "https://fixpull.test" || pr.ServerToken != "fixpull-token" || pr.BackendDisabled {
			t.Error("legacy import changed this device's server connection")
		}
		pushes <- req
		*out = backendrpc.PutWorkspaceResponse{Accepted: true, Version: 8, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		return nil
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(uic.CreateElement(hostedDataHydrationGate, HostedAuthGateProps{Content: Div(Attr("data-testid", "migration-budget"))})))
	f.Stabilize()
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	migrationWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationProtect })
	browserstore.Barrier()
	if !legacyWorkspaceNeedsProtection() || browserstore.GetString(datasetStoreKey) == "" || loadSyncMeta(activeWorkspaceID()).Version != 7 {
		t.Fatal("legacy data and its CAS version were not durable before the passcode prompt")
	}
	f.Stabilize()
	if hostedFirstOpenTestID(f, "hosted-hydration-lock") == nil || hostedFirstOpenTestID(f, "migration-budget") != nil {
		t.Fatal("loaded legacy budget did not enter the passcode form")
	}
	if loadSyncStatus().State != string(syncstate.StatusNeedsDataProtection) {
		t.Fatalf("legacy sync did not explain the App Lock requirement: %+v", loadSyncStatus())
	}
	// Submit through the hosted form, including the existing confirmation check.
	inputs := f.AllByTag("input")
	if len(inputs) != 2 {
		t.Fatal("missing passcode and confirmation fields")
	}
	inputs[0].Input("000000")
	inputs[1].Input("000000")
	f.AllByTag("button")[0].Click()
	f.Stabilize()
	if currentHostedHydration().Phase != hostedHydrationProtect || loadAppLock().Enabled {
		t.Fatal("weak passcode enabled App Lock")
	}
	inputs = f.AllByTag("input")
	inputs[0].Input("migration-passcode")
	inputs[1].Input("mistyped")
	f.AllByTag("button")[0].Click()
	f.Stabilize()
	if currentHostedHydration().Phase != hostedHydrationProtect || loadAppLock().Enabled {
		t.Fatal("mismatched passcode enabled App Lock")
	}
	f.AllByTag("input")[1].Input("migration-passcode")
	f.AllByTag("button")[0].Click()
	migrationWait(t, func() bool { return serverWorkspaceWasEncrypted(activeWorkspaceID()) && pendingSyncCount() == 0 })
	var req backendrpc.PutWorkspaceRequest
	select {
	case req = <-pushes:
	default:
		t.Fatal("enabling App Lock did not immediately upload")
	}
	if !cryptobox.IsEnvelope(req.Dataset) || req.Force || req.BaseVersion != 7 || req.Workspace.ID != activeWorkspaceID() {
		t.Fatalf("migration bypassed encrypted CAS push: %+v", req)
	}
	plain, err := decryptEnvelopeSync(req.Dataset, "migration-passcode")
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.Import(plain)
	if err != nil || len(after.Transactions) != len(before.Transactions) || len(after.Accounts) != len(before.Accounts) {
		t.Fatalf("encrypted replacement lost records: %v", err)
	}
	if !loadAppLock().Enabled || currentHostedHydration().Required || legacyWorkspaceNeedsProtection() {
		t.Fatal("accepted migration did not release the gate and clear legacy status")
	}
	// A later device state without App Lock still cannot accept plaintext for
	// the encrypted workspace. A refresh/reconnect must not re-import it.
	saveAppLock(applock.Config{})
	activePasscode = ""
	initializeHostedHydration(true)
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	if got := currentHostedHydration(); got.Phase != hostedHydrationError || !strings.Contains(got.Message, "unprotected copy") {
		t.Fatalf("accepted a downgrade without App Lock: %+v", got)
	}
}

func TestLegacyMigrationUploadFailureKeepsEncryptedRetry(t *testing.T) {
	migrationFixture(t)
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	migrationWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationProtect })
	hostedPushWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.PutWorkspaceRequest, _ *backendrpc.PutWorkspaceResponse) error {
		if !cryptobox.IsEnvelope(req.Dataset) || req.BaseVersion != 7 || req.Force {
			t.Error("failed migration did not use encrypted CAS")
		}
		return errors.New("offline for migration test")
	}
	enableHostedLegacyProtection("migration-passcode")
	migrationWait(t, func() bool { return loadSyncStatus().State == "error" && pendingSyncCount() == 1 })
	if serverWorkspaceWasEncrypted(activeWorkspaceID()) || !legacyWorkspaceNeedsProtection() {
		t.Fatal("failed upload was acknowledged as an encrypted server copy")
	}
	hostedPushWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.PutWorkspaceRequest, out *backendrpc.PutWorkspaceResponse) error {
		if !cryptobox.IsEnvelope(req.Dataset) || req.BaseVersion != 7 || req.Force {
			t.Error("retry changed encryption or CAS")
		}
		*out = backendrpc.PutWorkspaceResponse{Accepted: true, Version: 8, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		return nil
	}
	flushBackendSyncQueue()
	migrationWait(t, func() bool { return pendingSyncCount() == 0 && serverWorkspaceWasEncrypted(activeWorkspaceID()) })
}

func TestLegacyMigrationBannerNamesTheRequiredAction(t *testing.T) {
	migrationFixture(t)
	saveSyncMeta(activeWorkspaceID(), syncMeta{Owner: signedInUserID(), Server: currentDatasetServerID(), LegacyPlaintext: true})
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(uic.CreateElement(retrySaveBanner)))
	f.Stabilize()
	buttons := f.AllByTag("button")
	if len(buttons) != 1 || buttons[0].Text() != uistate.T("migration.enableLock") {
		t.Fatal("legacy save banner offered a generic Retry instead of App Lock")
	}
}

func TestLegacyMigrationRetainedLocalEditsStillPromptWithoutRebasing(t *testing.T) {
	migrationFixture(t)
	hadLocalDataset = true
	saveSyncMeta(activeWorkspaceID(), syncMeta{Owner: signedInUserID(), Version: 4, UpdatedAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)})
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	if currentHostedHydration().Phase != hostedHydrationProtect || !legacyWorkspaceNeedsProtection() || loadSyncMeta(activeWorkspaceID()).Version != 4 {
		t.Fatal("retained local work either skipped the migration prompt or was rebased onto the unread server version")
	}
}

func TestLegacyMigrationEncryptionLatchIsScoped(t *testing.T) {
	keys := map[string]bool{}
	for _, scope := range [][3]string{{"server-a", "user-a", "default"}, {"server-b", "user-a", "default"}, {"server-a", "user-b", "default"}, {"server-a", "user-a", "other"}, {"a:b", "c", "d"}, {"a", "b:c", "d"}} {
		key := encryptedWorkspaceKeyFor(scope[0], scope[1], scope[2])
		if keys[key] {
			t.Fatalf("encryption marker crossed scopes: %v", scope)
		}
		keys[key] = true
	}
}
