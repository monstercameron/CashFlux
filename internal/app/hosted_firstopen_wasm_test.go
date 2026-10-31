// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func hostedFirstOpenFixture(t *testing.T) {
	t.Helper()
	oldList, oldTimeout := hostedBootstrapWorkspaceListRPC, remoteWorkspaceListTimeout
	oldMerged, oldMerging, oldHadLocal := mergedRemoteWorkspaces, mergingRemoteWorkspaces, hadLocalDataset
	oldSample := uistate.SampleActive()
	pageRefreshState.Lock()
	oldScope, oldReady, oldBlocked := pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = nil, true, false
	pageRefreshState.Unlock()
	pr := prefs.Default()
	pr.ServerURL = "https://hosted-first-open.test"
	pr.ServerToken = "hosted-first-open-test-token"
	pr.BackendDisabled = false
	uistate.PersistPrefs(pr)
	mergedRemoteWorkspaces, mergingRemoteWorkspaces = false, false
	hadLocalDataset = true
	uistate.SetSampleActive(false)
	initializeHostedHydration(true)
	t.Cleanup(func() {
		hostedBootstrapWorkspaceListRPC, remoteWorkspaceListTimeout = oldList, oldTimeout
		mergedRemoteWorkspaces, mergingRemoteWorkspaces, hadLocalDataset = oldMerged, oldMerging, oldHadLocal
		uistate.SetSampleActive(oldSample)
		initializeHostedHydration(false)
		pageRefreshState.Lock()
		pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = oldScope, oldReady, oldBlocked
		pageRefreshState.Unlock()
	})
}

func hostedFirstOpenWait(t *testing.T, test func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if test() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("hosted first-open RPC did not reach its expected state")
}

func TestHostedFirstOpenEmptyWorkspaceListOffersStartStep(t *testing.T) {
	hostedFirstOpenFixture(t)
	calls := 0
	hostedBootstrapWorkspaceListRPC = func(context.Context, prefs.Prefs, *backendrpc.ListWorkspacesResponse) error {
		calls++
		return nil
	}
	done := false
	mergeRemoteWorkspaces(func() { done = true })
	hostedFirstOpenWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationEmpty })
	if calls != 1 || done {
		t.Fatalf("empty list calls=%d pull-sequencing callback=%v, want one list and no pull", calls, done)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("hosted-empty-test"), uic.CreateElement(hostedDataHydrationGate, HostedAuthGateProps{Content: Div()})))
	f.Stabilize()
	if len(f.AllByTag("button")) != 2 || hostedFirstOpenTestID(f, "hosted-start-budget") == nil || hostedFirstOpenTestID(f, "hosted-import-backup") == nil {
		t.Fatal("empty hosted account did not show Start your budget and Import a backup actions")
	}
}

func TestHostedFirstOpenWorkspaceListErrorShowsRetry(t *testing.T) {
	hostedFirstOpenFixture(t)
	calls := 0
	hostedBootstrapWorkspaceListRPC = func(context.Context, prefs.Prefs, *backendrpc.ListWorkspacesResponse) error {
		calls++
		if calls == 1 {
			return errors.New("service unavailable")
		}
		return nil
	}
	done := false
	mergeRemoteWorkspaces(func() { done = true })
	hostedFirstOpenWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationError })
	status := currentHostedHydration()
	if done || !status.RetryWorkspaceList || status.Message == "" {
		t.Fatalf("list failure state = %#v, pull callback=%v", status, done)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("hosted-error-test"), uic.CreateElement(hostedDataHydrationGate, HostedAuthGateProps{Content: Div()})))
	f.Stabilize()
	if hostedFirstOpenTestID(f, "hosted-hydration-error") == nil {
		t.Fatal("workspace-list error is not presented as an actionable error")
	}
	buttons := f.AllByTag("button")
	if len(buttons) != 1 || buttons[0].Text() != uistate.T("hostedHydration.retry") {
		t.Fatal("workspace-list error did not show Retry")
	}
	buttons[0].Click()
	f.Stabilize()
	hostedFirstOpenWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationEmpty })
	if calls != 2 {
		t.Fatalf("Retry made %d workspace-list calls, want 2", calls)
	}
}

func TestHostedFirstOpenExistingWorkspaceContinuesToPull(t *testing.T) {
	hostedFirstOpenFixture(t)
	hostedBootstrapWorkspaceListRPC = func(_ context.Context, _ prefs.Prefs, out *backendrpc.ListWorkspacesResponse) error {
		out.Workspaces = []backendrpc.Workspace{{ID: "hosted-first-open-existing", Name: "Family"}}
		return nil
	}
	continued := make(chan struct{}, 1)
	mergeRemoteWorkspaces(func() { continued <- struct{}{} })
	hostedFirstOpenWait(t, func() bool {
		select {
		case <-continued:
			return true
		default:
			return false
		}
	})
	if !loadRegistry().Has("hosted-first-open-existing") {
		t.Fatal("existing server workspace was not reconciled before the pull callback")
	}
	if currentHostedHydration().Phase == hostedHydrationEmpty || currentHostedHydration().Phase == hostedHydrationError {
		t.Fatalf("existing workspace entered an empty/error step: %#v", currentHostedHydration())
	}
}

func TestHostedFirstOpenWorkspaceListTimeoutShowsRetry(t *testing.T) {
	hostedFirstOpenFixture(t)
	remoteWorkspaceListTimeout = 10 * time.Millisecond
	hostedBootstrapWorkspaceListRPC = func(ctx context.Context, _ prefs.Prefs, _ *backendrpc.ListWorkspacesResponse) error {
		<-ctx.Done()
		return ctx.Err()
	}
	mergeRemoteWorkspaces(nil)
	hostedFirstOpenWait(t, func() bool { return currentHostedHydration().Phase == hostedHydrationError })
	if !currentHostedHydration().RetryWorkspaceList {
		t.Fatal("workspace-list timeout did not enable retry")
	}
}

func hostedFirstOpenTestID(f *render.Fixture, id string) *render.QueryNode {
	for _, tag := range []string{"div", "button"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == id && id != "" {
				return node
			}
		}
	}
	return nil
}
