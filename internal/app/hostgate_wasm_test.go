// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func hostgateEnvelope() []byte {
	return cryptobox.Marshal(cryptobox.Envelope{
		V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM,
		Salt: "c2FsdA==", IV: "aXY=", Cipher: "Y2lwaGVy",
	})
}

func hostgateWaitPull(t *testing.T) {
	t.Helper()
	fixpullWait(t, func() bool {
		pullMu.Lock()
		defer pullMu.Unlock()
		return !pullInFlight
	})
}

func TestHOSTGATEPullUpdatesMountedLoadingGate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase hostedHydrationPhase
		id    string
	}{
		{"encrypted", hostedHydrationLocked, "hosted-hydration-lock"},
		{"missing", hostedHydrationEmpty, "hosted-hydration-empty-account"},
		{"empty snapshot", hostedHydrationEmpty, "hosted-hydration-empty-account"},
		{"scope discard", hostedHydrationReady, "hostgate-content"},
		{"timeout", hostedHydrationError, "hosted-hydration-error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixpullFixture(t)
			hadLocalDataset = false
			oldRPC := hostedPullWorkspaceRPC
			t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
			hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
				switch tc.name {
				case "timeout":
					return context.DeadlineExceeded
				case "missing":
					return nil
				case "empty snapshot":
					out.Found, out.Workspace.ID = true, req.ID
				case "scope discard":
					out.Found, out.Workspace.ID, out.Dataset = true, "another-workspace", hostgateEnvelope()
				default:
					out.Found, out.Workspace.ID, out.Dataset = true, req.ID, hostgateEnvelope()
				}
				return nil
			}
			f := render.New(t)
			defer f.Cleanup()
			f.Render(Body(uic.CreateElement(hostedDataHydrationGate, HostedAuthGateProps{
				Content: Div(Attr("data-testid", "hostgate-content"), "Budget"),
			})))
			f.Stabilize()
			if hostedFirstOpenTestID(f, "hosted-hydration-loading") == nil {
				t.Fatal("fixture did not mount the initial loading screen")
			}
			pullActiveWorkspaceFromBackend(false)
			hostgateWaitPull(t)
			f.Stabilize()
			if got := currentHostedHydration(); got.Phase != tc.phase {
				t.Fatalf("hydration = %#v, want phase %s", got, tc.phase)
			}
			if hostedFirstOpenTestID(f, tc.id) == nil || hostedFirstOpenTestID(f, "hosted-hydration-loading") != nil {
				t.Fatalf("mounted gate did not replace Loading with %s", tc.id)
			}
			if tc.name == "encrypted" {
				got := currentHostedHydration()
				if !got.Required || !bytes.Equal(got.EncryptedSnapshot, hostgateEnvelope()) {
					t.Fatalf("encrypted pull did not retain the required lock: %#v", got)
				}
			}
			if tc.name == "timeout" && !strings.Contains(currentHostedHydration().Message, "deadline exceeded") {
				t.Fatal("timeout cause was lost")
			}
			if tc.name == "timeout" {
				buttons := f.AllByTag("button")
				if len(buttons) != 1 {
					t.Fatal("transport error did not show a retry action")
				}
				calls := 0
				hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
					calls++
					out.Found, out.Workspace.ID, out.Dataset = true, req.ID, hostgateEnvelope()
					return nil
				}
				buttons[0].Click()
				hostgateWaitPull(t)
				f.Stabilize()
				if calls != 1 || hostedFirstOpenTestID(f, "hosted-hydration-lock") == nil {
					t.Fatal("Retry did not replace the timeout with the server's passcode form")
				}
			}
		})
	}
}

func TestHOSTGATEBackgroundPullPreservesTerminalPhase(t *testing.T) {
	for _, phase := range []hostedHydrationPhase{hostedHydrationLocked, hostedHydrationEmpty, hostedHydrationError, hostedHydrationReady, hostedHydrationVerifying, hostedHydrationApplying} {
		t.Run(string(phase), func(t *testing.T) {
			fixpullFixture(t)
			setHostedHydration(hostedHydrationStatus{
				Required: phase != hostedHydrationReady, Phase: phase,
				Message: "Keep this result", EncryptedSnapshot: hostgateEnvelope(),
			})
			oldRPC := hostedPullWorkspaceRPC
			t.Cleanup(func() { hostedPullWorkspaceRPC = oldRPC })
			calls := 0
			hostedPullWorkspaceRPC = func(context.Context, prefs.Prefs, backendrpc.GetWorkspaceRequest, *backendrpc.GetWorkspaceResponse) error {
				calls++
				return context.DeadlineExceeded
			}
			for range 3 {
				markHostedHydrationLoading()
				pullActiveWorkspaceFromBackend(false)
				got := currentHostedHydration().Phase
				hostgateWaitPull(t)
				if got != phase || currentHostedHydration().Phase != phase {
					t.Fatalf("background trigger reset %s to %s", phase, got)
				}
			}
			if phase != hostedHydrationReady && calls != 0 {
				t.Fatalf("terminal gate made %d unsolicited RPCs", calls)
			}
		})
	}
}

func TestHOSTGATEPullPreconditionsShowError(t *testing.T) {
	for _, name := range []string{"session unavailable", "connection disabled", "no active workspace"} {
		t.Run(name, func(t *testing.T) {
			fixpullFixture(t)
			switch name {
			case "session unavailable":
				pageRefreshState.Lock()
				pageRefreshState.blocked = true
				pageRefreshState.Unlock()
			case "connection disabled":
				pr := uistate.LoadPrefs()
				pr.BackendDisabled = true
				uistate.PersistPrefs(pr)
			case "no active workspace":
				lsRemove(workspacesKey)
			}
			pullActiveWorkspaceFromBackend(false)
			if got := currentHostedHydration(); got.Phase != hostedHydrationError || !got.Required || got.Message == "" {
				t.Fatalf("unavailable pull left no actionable error: %#v", got)
			}
		})
	}
}

func TestHOSTGATEReconnectWaitsForWorkspaceList(t *testing.T) {
	fixpullFixture(t)
	oldMerging, oldRPC := mergingRemoteWorkspaces, hostedPullWorkspaceRPC
	t.Cleanup(func() { mergingRemoteWorkspaces, hostedPullWorkspaceRPC = oldMerging, oldRPC })
	calls := 0
	hostedPullWorkspaceRPC = func(_ context.Context, _ prefs.Prefs, req backendrpc.GetWorkspaceRequest, out *backendrpc.GetWorkspaceResponse) error {
		calls++
		out.Found, out.Workspace.ID, out.Dataset = true, req.ID, hostgateEnvelope()
		return nil
	}
	mergingRemoteWorkspaces = true
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	if calls != 0 || currentHostedHydration().Phase != hostedHydrationWaiting {
		t.Fatal("reconnect raced ListWorkspaces and pulled the unadopted workspace")
	}
	mergedRemoteWorkspaces = true
	pullActiveWorkspaceFromBackend(false)
	hostgateWaitPull(t)
	if calls != 1 || currentHostedHydration().Phase != hostedHydrationLocked {
		t.Fatal("workspace-list completion did not continue to the required lock")
	}
}
