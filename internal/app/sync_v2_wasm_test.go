// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	uic "github.com/monstercameron/GoWebComponents/v8/ui"
)

func TestSyncV2StatusLabelsAndFacesHandleLockedAndProtection(t *testing.T) {
	for _, state := range []string{"locked", "needs-data-protection"} {
		key, cls, ok := syncChipFace(state)
		if !ok || key == "" || cls != "sync-chip sync-warn" {
			t.Fatalf("syncChipFace(%q) = (%q, %q, %v), want warn chip", state, key, cls, ok)
		}
		title, hint := cloudFace(cloudFacts{SignedIn: true, Protected: true, State: state}, cloudDone)
		if title != "cloud1.attention" || hint != "cloud1.protectHint" {
			t.Fatalf("cloudFace(%q) = (%q, %q), want attention/protectHint", state, title, hint)
		}
	}
}

func TestCloudConflictReviewRendersV2DiffAndMergeAction(t *testing.T) {
	merged := 0
	cmp := conflictcheckpoint.Comparison{
		Available: true,
		Counts: map[conflictcheckpoint.Side]map[string]int{
			conflictcheckpoint.SideLocal:  {"transactions": 12},
			conflictcheckpoint.SideServer: {"transactions": 11},
		},
		Changes: []conflictcheckpoint.RecordChange{
			{Kind: conflictcheckpoint.ChangeChanged, Entity: "transaction", ID: "tx-1", BeforeLabel: "Coffee", AfterLabel: "Cafe"},
		},
	}
	f := cloudTestFixture(t, uic.CreateElement(cloudConflictReview, cloudConflictReviewProps{
		Open:        true,
		HasDiff:     true,
		Comparison:  cmp,
		OnClose:     func() {},
		OnMerge:     func() { merged++ },
		OnKeepLocal: func() {},
		OnUseServer: func() {},
	}))
	if cloudTestID(f, "div", "sync-conflict-diff") == nil {
		t.Fatal("expected sync-conflict-diff summary card")
	}
	btn := cloudTestID(f, "button", "sync-conflict-merge")
	if btn == nil {
		t.Fatal("expected sync-conflict-merge button")
	}
	btn.Click()
	f.Stabilize()
	if merged != 1 {
		t.Fatalf("OnMerge calls = %d, want 1", merged)
	}
}

func TestHostedHydrationWakeSyncAndManualSyncDoNotDeadlockOnPushBlocked(t *testing.T) {
	fixpullFixture(t)
	oldPull, oldWake := hostedPullWorkspaceRPC, lastWakeAt
	t.Cleanup(func() {
		hostedPullWorkspaceRPC = oldPull
		wakeMu.Lock()
		lastWakeAt = oldWake
		wakeMu.Unlock()
	})
	calls := 0
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		calls++
		out.Found, out.Workspace.ID, out.Dataset = true, req.ID, hostgateEnvelope()
		return nil
	}
	mergedRemoteWorkspaces = true
	wakeMu.Lock()
	lastWakeAt = time.Time{}
	wakeMu.Unlock()

	wakeSync()
	hostgateWaitPull(t)
	if calls != 1 || currentHostedHydration().Phase != hostedHydrationLocked {
		t.Fatalf("wakeSync during hosted hydration: calls=%d phase=%s, want 1 / %s", calls, currentHostedHydration().Phase, hostedHydrationLocked)
	}

	// Reset back to waiting and verify requestBackendSyncNow also drives hosted hydration.
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationWaiting})
	requestBackendSyncNow()
	hostgateWaitPull(t)
	if calls != 2 || currentHostedHydration().Phase != hostedHydrationLocked {
		t.Fatalf("requestBackendSyncNow during hosted hydration: calls=%d phase=%s, want 2 / %s", calls, currentHostedHydration().Phase, hostedHydrationLocked)
	}
}

func TestIdleAutosaveDuringHostedHydrationDoesNotRaiseUnsavedBanner(t *testing.T) {
	oldHosted := currentHostedHydration()
	oldApp := appstate.Default
	t.Cleanup(func() {
		setHostedHydration(oldHosted)
		appstate.Default = oldApp
		uistate.ClearPersistFailure("dataset")
	})

	if err := appstate.Init(nil, false); err != nil {
		t.Fatalf("appstate.Init: %v", err)
	}
	app := appstate.Default
	t.Cleanup(func() { _ = app.Store().Close() })

	uistate.ClearPersistFailure("dataset")
	initializeHostedHydration(true)

	save, _ := newRecordAutosave(app, func(_ map[string]string, _ map[string]string, cb func(error)) {
		cb(nil)
	}, nil)

	f := cloudTestFixture(t, uic.CreateElement(SaveFailureBanner))
	// Idle boot/ticker save while hosted hydration is still checking the server
	// must stay silent and not flash "Changes aren't saved yet".
	save()
	f.Stabilize()
	if cloudTestID(f, "div", "unsaved-changes") != nil {
		t.Fatal("idle autosave during hosted hydration raised persist failure banner")
	}

	// An explicit durable writer request must still fail loudly while paused.
	done := make(chan appstate.DurableResult, 1)
	app.RequestDurable(func(res appstate.DurableResult) { done <- res })
	select {
	case res := <-done:
		if res.Err == nil {
			t.Fatal("explicit durable request succeeded while hosted hydration was required")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("explicit durable request timed out")
	}
}
