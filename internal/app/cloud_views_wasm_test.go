// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func cloudTestFixture(t *testing.T, node uic.Node) *render.Fixture {
	t.Helper()
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	f.Render(Body(ID("cloud-test-body"), node))
	f.Stabilize()
	return f
}

func cloudTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, n := range f.AllByTag(tag) {
		if n.Attr("data-testid") == id {
			return n
		}
	}
	return nil
}

func TestCloudStatusStatesAndLiveRegion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts cloudFacts
		step  cloudStep
		title string
	}{
		{"off", cloudFacts{Disabled: true}, cloudOff, "cloud1.off"},
		{"setup", cloudFacts{Disabled: true, Setup: "where"}, cloudWhere, "cloud1.settingUp"},
		{"synced", cloudFacts{SignedIn: true, Protected: true, State: "synced"}, cloudDone, "cloud1.synced"},
		{"attention", cloudFacts{SignedIn: true, Protected: true, State: "error"}, cloudDone, "cloud1.attention"},
		{"paused", cloudFacts{SignedIn: true, Protected: true, State: "offline"}, cloudDone, "cloud1.paused"},
		{"conflict", cloudFacts{SignedIn: true, Protected: true, State: "conflict"}, cloudDone, "cloud1.attention"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := cloudTestFixture(t, uic.CreateElement(cloudStatusView, cloudStatusProps{Facts: tc.facts, Step: tc.step, Device: "Test device"}))
			if len(f.AllByTag("h2")) != 1 || f.AllByTag("h2")[0].Text() != uistate.T(tc.title) {
				t.Fatal("state has no accessible heading")
			}
			live := false
			for _, node := range f.AllByTag("div") {
				if node.Attr("role") == "status" && node.Attr("aria-live") == "polite" {
					live = true
				}
			}
			if !live {
				t.Fatal("state changes are not politely announced")
			}
		})
	}
}

func TestCloudSetupShowsOnlyTheCurrentTask(t *testing.T) {
	for _, step := range []cloudStep{cloudWhere, cloudSignIn, cloudProtect, cloudFirst} {
		t.Run(string(step), func(t *testing.T) {
			f := cloudTestFixture(t, uic.CreateElement(cloudSetupView, cloudSetupProps{Step: step,
				Where: Input(Attr("data-testid", "where")), SignIn: Input(Attr("data-testid", "signin")),
				Protect: Input(Attr("data-testid", "protect")), First: Input(Attr("data-testid", "first"))}))
			if len(f.AllByTag("input")) != 1 || cloudTestID(f, "input", string(step)) == nil {
				t.Fatal("setup exposes another task's controls")
			}
			current := 0
			for _, node := range f.AllByTag("li") {
				if node.Attr("aria-current") == "step" {
					current++
				}
			}
			if current != 1 {
				t.Fatalf("current steps = %d, want one", current)
			}
		})
	}
}

func TestCloudFirstSyncCannotStartWithoutEncryption(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		t.Run(string(rune('0'+mask)), func(t *testing.T) {
			started := 0
			facts := cloudFacts{Disabled: mask&1 != 0, SignedIn: mask&2 != 0, Protected: mask&4 != 0}
			f := cloudTestFixture(t, uic.CreateElement(cloudFirstView, cloudFirstProps{Facts: facts, OnStart: func() { started++ }}))
			button := cloudTestID(f, "button", "cloud-start-encrypted-sync")
			button.Click()
			f.Stabilize()
			want := 0
			if mask == 7 {
				want = 1
			}
			if started != want {
				t.Fatalf("start callbacks = %d, want %d for %#v", started, want, facts)
			}
		})
	}
}

func TestCloudFirstSyncOffersRetryAfterConnectionFailure(t *testing.T) {
	calls := 0
	f := cloudTestFixture(t, uic.CreateElement(cloudFirstView, cloudFirstProps{
		Facts: cloudFacts{SignedIn: true, Protected: true, State: "error"}, OnRetry: func() { calls++ },
	}))
	cloudTestID(f, "button", "cloud-first-retry").Click()
	f.Stabilize()
	if calls != 1 {
		t.Fatal("failed first sync cannot be retried")
	}
}

func TestCloudOAuthCannotAuthorizeUnprotectedSync(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(map[bool]string{false: "unprotected", true: "protected"}[protected], func(t *testing.T) {
			calls := 0
			f := cloudTestFixture(t, uic.CreateElement(cloudSignInView, cloudSignInProps{Connection: cloudConnection{Segment: "commercial"}, Protected: protected,
				Auth: cloudAuthActions{Google: func() { calls++ }}}))
			cloudTestID(f, "button", "sync-oauth-google").Click()
			f.Stabilize()
			if calls != map[bool]int{false: 0, true: 1}[protected] {
				t.Fatal("OAuth bypassed protection")
			}
		})
	}
}

func TestCloudConnectionRequiresSuccessfulCheck(t *testing.T) {
	for _, phase := range []discoveryPhase{discoveryIdle, discoveryChecking, discoveryError, discoveryOK} {
		t.Run(string(phase), func(t *testing.T) {
			cloudTestMeasurementDOM(t)
			continued := 0
			f := cloudTestFixture(t, uic.CreateElement(cloudWhereView, cloudWhereProps{Connection: cloudConnection{Segment: "remote", Phase: phase}, OnContinue: func() { continued++ }}))
			cloudTestID(f, "button", "cloud-where-continue").Click()
			f.Stabilize()
			if continued != map[bool]int{false: 0, true: 1}[phase == discoveryOK] {
				t.Fatal("unchecked connection passed setup")
			}
			field := cloudTestID(f, "input", "sync-server-url")
			if field == nil || field.Attr("id") == "" || field.Attr("aria-describedby") == "" {
				t.Fatal("address lacks its visible label or hint relationship")
			}
		})
	}
}

func TestCloudDangerRequiresNamedConfirmationAndWaitsForCompletion(t *testing.T) {
	for _, mode := range []string{"pause", "delete"} {
		t.Run(mode, func(t *testing.T) {
			calls, closed := 0, 0
			var complete func(error)
			run := func(done func(error)) { calls++; complete = done }
			f := cloudTestFixture(t, uic.CreateElement(cloudDangerConfirm, cloudDangerConfirmProps{Mode: mode, Household: "Family", Server: "example.test", OnClose: func() { closed++ }, OnPause: run, OnDelete: run}))
			if f.ByRole("dialog", uistate.T("cloud1."+map[string]string{"pause": "pause", "delete": "delete"}[mode])) == nil {
				t.Fatal("confirmation lacks its named dialog")
			}
			if f.BuildOverlayFocusSurfaceID() == "" || f.BuildOverlayEscapeSurfaceID() == "" {
				t.Fatal("confirmation lacks focus/Escape ownership")
			}
			cloudTestID(f, "button", "cloud-danger-submit").Click()
			f.Stabilize()
			if calls != 0 {
				t.Fatal("unnamed confirmation executed")
			}
			cloudTestID(f, "input", "cloud-confirm-name").Input("Another household")
			f.Stabilize()
			cloudTestID(f, "button", "cloud-danger-submit").Click()
			f.Stabilize()
			if calls != 0 {
				t.Fatal("wrong name executed")
			}
			cloudTestID(f, "input", "cloud-confirm-name").Input("Family")
			f.Stabilize()
			cloudTestID(f, "button", "cloud-danger-submit").Click()
			f.Stabilize()
			if calls != 1 || closed != 0 {
				t.Fatal("confirmation closed before durable completion")
			}
			complete(errors.New("disk full"))
			f.Stabilize()
			if closed != 0 || f.ByRole("alert", "") == nil {
				t.Fatal("failed action hid its error or closed")
			}
			cloudTestID(f, "button", "cloud-danger-submit").Click()
			f.Stabilize()
			complete(nil)
			f.Stabilize()
			if calls != 2 || closed != 1 {
				t.Fatal("successful completion did not close exactly once")
			}
		})
	}
}

func TestCloudDangerCancelAndBackdropDoNotWrite(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "backdrop"}[escape], func(t *testing.T) {
			calls, closed := 0, 0
			f := cloudTestFixture(t, uic.CreateElement(cloudDangerConfirm, cloudDangerConfirmProps{Mode: "delete", Household: "Family", OnClose: func() { closed++ }, OnDelete: func(func(error)) { calls++ }}))
			if escape {
				f.HandleOverlayOutsideClick(f.BuildOverlaySurfaces()[0].SurfaceID)
			} else {
				cloudTestID(f, "button", "cloud-danger-cancel").Click()
			}
			f.Stabilize()
			if calls != 0 || closed != 1 {
				t.Fatal("dismissal executed a destructive action")
			}
		})
	}
}

func TestCloudDangerConfirmationKeepsTheOpenedHouseholdAndAction(t *testing.T) {
	a, b := 0, 0
	first := cloudDangerProps{Household: "Family A", Server: "server-a.test", OnDelete: func(done func(error)) { a++; done(nil) }}
	f := cloudTestFixture(t, uic.CreateElement(cloudDangerView, first))
	cloudTestID(f, "button", "cloud-delete-server").Click()
	f.Stabilize()
	second := cloudDangerProps{Household: "Family B", Server: "server-b.test", OnDelete: func(done func(error)) { b++; done(nil) }}
	f.Render(Body(ID("cloud-test-body"), uic.CreateElement(cloudDangerView, second)))
	f.Stabilize()
	cloudTestID(f, "input", "cloud-confirm-name").Input("Family A")
	f.Stabilize()
	cloudTestID(f, "button", "cloud-danger-submit").Click()
	f.Stabilize()
	if a != 1 || b != 0 {
		t.Fatal("an open named confirmation changed its household/action scope")
	}
}

func TestCloudConflictDefaultsToKeepingBoth(t *testing.T) {
	closed, local, server := 0, 0, 0
	f := cloudTestFixture(t, uic.CreateElement(cloudConflictTestHost, cloudConflictReviewProps{OnClose: func() { closed++ }, OnKeepLocal: func() { local++ }, OnUseServer: func() { server++ }}))
	cloudTestID(f, "button", "cloud-conflicts-entry").Click()
	f.Stabilize()
	if f.ByRole("dialog", uistate.T("cloud1.copiesTitle")) == nil {
		t.Fatal("conflict entry did not open accessible review")
	}
	cloudTestID(f, "button", "sync-conflict-keep-both").Click()
	f.Stabilize()
	if closed != 1 || local != 0 || server != 0 {
		t.Fatal("keep both replaced or discarded a copy")
	}
}

func cloudConflictTestHost(p cloudConflictReviewProps) uic.Node {
	open := uic.UseState(false)
	p.Open = open.Get()
	onClose := p.OnClose
	p.OnClose = func() { open.Set(false); onClose() }
	return Div(uic.CreateElement(cloudConflictEntry, cloudConflictEntryProps{OnReview: func() { open.Set(true) }}), uic.CreateElement(cloudConflictReview, p))
}

func TestCloudDevicesMarksThisDeviceAndEmitsRevoke(t *testing.T) {
	var revoked string
	f := cloudTestFixture(t, uic.CreateElement(cloudDevicesView, cloudDevicesProps{Phase: cloudDevicesReady,
		Devices:  []backendrpc.DeviceSession{{FamilyID: "here", DeviceLabel: "Laptop", Current: true}, {FamilyID: "other", DeviceLabel: "Phone"}},
		OnRevoke: func(d backendrpc.DeviceSession) { revoked = d.FamilyID }}))
	if cloudTestID(f, "button", "cloud-revoke-here") != nil {
		t.Fatal("current device offered remote revoke")
	}
	cloudTestID(f, "button", "cloud-revoke-other").Click()
	f.Stabilize()
	if revoked != "other" {
		t.Fatal("revoke callback has the wrong device scope")
	}
	if f.ByRole("status", uistate.T("devices.current")) == nil {
		t.Fatal("current device is not marked")
	}
}

func TestCloudDeviceListFailureOffersRetry(t *testing.T) {
	calls := 0
	f := cloudTestFixture(t, uic.CreateElement(cloudDevicesView, cloudDevicesProps{Phase: cloudDevicesFailed, OnRetry: func() { calls++ }}))
	if f.ByRole("alert", "") == nil {
		t.Fatal("device list failure is not announced")
	}
	cloudTestID(f, "button", "cloud-devices-retry").Click()
	f.Stabilize()
	if calls != 1 {
		t.Fatal("device list failure has no recovery action")
	}
}

func TestCloudSetupFocusMovesOnlyWhenTheTaskChanges(t *testing.T) {
	previous := js.Global().Get("document")
	doc, section := js.Global().Get("Object").New(), js.Global().Get("Object").New()
	focused := 0
	focus := js.FuncOf(func(js.Value, []js.Value) any { focused++; return nil })
	lookup := js.FuncOf(func(js.Value, []js.Value) any { return section })
	section.Set("focus", focus)
	doc.Set("getElementById", lookup)
	js.Global().Set("document", doc)
	t.Cleanup(func() { js.Global().Set("document", previous); focus.Release(); lookup.Release() })
	f := cloudTestFixture(t, uic.CreateElement(cloudSetupView, cloudSetupProps{Step: cloudWhere}))
	if focused != 1 {
		t.Fatal("setup did not focus its current task")
	}
	f.Render(Body(ID("cloud-test-body"), uic.CreateElement(cloudSetupView, cloudSetupProps{Step: cloudWhere})))
	f.Stabilize()
	if focused != 1 {
		t.Fatal("ordinary rerender stole input focus")
	}
	f.Render(Body(ID("cloud-test-body"), uic.CreateElement(cloudSetupView, cloudSetupProps{Step: cloudSignIn})))
	f.Stabilize()
	if focused != 2 {
		t.Fatal("next task did not receive keyboard focus")
	}
}

// Segmented's browser measurement effect has no real elements in testkit.
// Each test owns and releases this tiny DOM seam; no app/store globals are used.
func cloudTestMeasurementDOM(t *testing.T) {
	t.Helper()
	enc31FakeDocument(t)
	doc := js.Global().Get("document")
	// Segmented's measurement lookup intentionally finds no real element in
	// testkit, while the framework CSS sink still needs usable style elements.
	lookup := js.FuncOf(func(js.Value, []js.Value) any { return js.Null() })
	listen := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	doc.Set("getElementById", lookup)
	doc.Set("querySelector", lookup)
	doc.Set("addEventListener", listen)
	doc.Set("head", doc.Get("createElement").Invoke("head"))
	doc.Set("documentElement", doc.Get("createElement").Invoke("html"))
	t.Cleanup(func() { lookup.Release(); listen.Release() })
}
