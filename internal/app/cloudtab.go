// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"time"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// currentPageOrigin returns the browser's document origin (e.g.
// "https://earlcameron.com"), matching the pattern already used by
// anchorintercept.go. Used to auto-detect a same-origin backend — the
// embedded-in-another-site case (e.g. CashFlux mounted at /budget/ with its
// sync bridge at /grpc on the SAME host) where the user should never have to
// type a server address at all.
func currentPageOrigin() string {
	return js.Global().Get("location").Get("origin").String()
}

// The screens registry can't import this package (app imports screens), so the
// /sync route's view is injected at boot — mirroring settings_route.go.
func init() {
	screens.SyncView = func() uic.Node { return uic.CreateElement(SyncRedirect) }
}

// SyncRedirect is the entire body of the routed /sync page now that Cloud
// connection UI lives in Settings → Cloud: /sync exists only so old bookmarks
// and links still land somewhere, not as a second implementation of the same
// feature (2026-07-24 unification). It navigates on to /settings/cloud
// immediately and renders nothing itself.
func SyncRedirect() uic.Node {
	uic.UseEffect(func() func() {
		uistate.NavigateTo("/settings/cloud")
		return nil
	}, "sync-redirect")
	return Fragment()
}

// discoveryPhase tracks the explicit or returning-device connection check.
type discoveryPhase string

const (
	discoveryIdle     discoveryPhase = "idle"
	discoveryChecking discoveryPhase = "checking"
	discoveryOK       discoveryPhase = "ok"
	discoveryError    discoveryPhase = "error"
)

// segmentFor derives the Local/Remote/Commercial selector value from
// persisted prefs, so a returning user lands back on the segment they chose.
func segmentFor(p prefs.Prefs) string {
	if p.ServerMode == prefs.ServerCloud {
		return "commercial"
	}
	return string(p.Normalize().ConnectionSegment)
}

// CloudConnectionPane composes the encrypted-sync setup and management views.
// Transport/authentication remain in their existing services; presentation only
// receives snapshots and callbacks. No action in this pane enables plaintext sync.
func CloudConnectionPane() uic.Node {
	prefsAtom := uistate.UsePrefs()
	rev := uistate.UseDataRevision()
	_ = rev.Get()
	_ = state.UseAtom(syncRevAtomID, 0).Get()
	_ = state.UseAtom(syncClockAtomID, 0).Get()
	uistate.CaptureAuthRevision(uistate.UseAuthRevision())
	pr := prefsAtom.Get().Normalize()
	hosted := hostedDocument()
	status := loadSyncStatus()
	signedIn := uistate.Session(pr.ServerToken).Present() && !status.AuthFailed
	protected := datasetEncryptionActive()
	workspaceID := activeWorkspaceID()
	bump := func() { rev.Update(func(n int) int { return n + 1 }) }
	setup := useCloudSetup(prefsAtom, status, signedIn, protected, bump)
	connection := useCloudServer(prefsAtom, setup.Step, !hosted)
	auth := useCloudAuth(prefsAtom, connection.URL, bump)
	devices := useCloudDevices(pr, signedIn)
	facts := cloudFacts{Disabled: pr.BackendDisabled, SignedIn: signedIn, Protected: protected, Hosted: hosted,
		Setup: setup.Saved, State: status.State, Pending: status.Pending}
	step := cloudNextStep(facts)
	action := cloudActionProps{Label: uistate.T("cloud1.syncNow"), TestID: "sync-now", Kind: cloudActionPrimary, OnClick: setup.SyncNow, Disabled: !cloudCanSync(facts) || setup.Busy}
	switch {
	case step == cloudOff && !hosted:
		action = cloudActionProps{Label: uistate.T("cloud1.start"), TestID: "cloud-setup-start", Kind: cloudActionPrimary, OnClick: setup.Start, Disabled: setup.Busy}
	case status.State == "conflict":
		action = cloudActionProps{Label: uistate.T("cloud1.review"), TestID: "cloud-review-conflicts", OnClick: openSyncConflict}
	case status.State == syncStateIdentityUnknown:
		action = cloudActionProps{Label: uistate.T("cloud1.signInAgain"), TestID: "cloud-signin-again", OnClick: auth.SignOut}
	case step == cloudProtect || status.State == "locked" || status.State == syncStateLockedDifferentPasscode:
		action = cloudActionProps{Label: uistate.T("cloud1.protectAction"), TestID: "sync-locked-enable-passcode", OnClick: setup.Protect}
	case step == cloudWhere || step == cloudSignIn || step == cloudFirst:
		action = cloudActionProps{}
	}
	var statusAction uic.Node
	if action.Label != "" {
		statusAction = uic.CreateElement(cloudAction, action)
	}
	peers := ""
	if devices.Phase == cloudDevicesReady && len(devices.Devices) > 0 {
		peers = uistate.T("cloud1.otherDevices", devices.OtherCount)
	}
	protection := uic.CreateElement(cloudProtectionView, cloudProtectionProps{Protected: protected, OnProtect: setup.Protect, OnPasskey: showPasskeyManager, OnBackup: backupEverythingEncrypted})
	where := uic.CreateElement(cloudWhereView, cloudWhereProps{Connection: connection, OnContinue: setup.Continue, Busy: setup.Busy})
	signin := uic.CreateElement(cloudSignInView, cloudSignInProps{Connection: connection, Auth: auth, Protected: protected, OnProtect: setup.Protect})
	first := uic.CreateElement(cloudFirstView, cloudFirstProps{Facts: facts, Busy: setup.Busy, OnStart: setup.Enable, OnRetry: setup.SyncNow})
	return Div(css.Class("cloud-tab", tw.Flex, tw.FlexCol, tw.Gap4),
		H2(css.Class(tw.Text18, tw.FontSemibold), uistate.T("cloud1.title")),
		uic.CreateElement(cloudStatusView, cloudStatusProps{Facts: facts, Step: step, LastSync: syncLastSyncedLabel(status.LastSyncedAt, time.Now()), Device: customSyncDeviceLabel(), Peers: peers, Action: statusAction}),
		If(setup.Error != "", P(css.Class(tw.Text14, tw.TextDanger), Attr("role", "alert"), setup.Error)),
		If(!hosted && status.AuthFailed && uistate.Session(pr.ServerToken).Present(), uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.clearInvalidToken"), TestID: "settings-clear-invalid-token", OnClick: auth.SignOut})),
		uic.CreateElement(RebindCard, rebindProps{ServerURL: pr.ServerURL, Status: status, OnSignOut: auth.SignOut, OnDisconnect: setup.Pause, OnRetry: setup.SyncNow}),
		If(status.State == syncStateLockedDifferentPasscode, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("sync.replaceServerCopyAction"), TestID: "sync-replace-server-copy", Kind: cloudActionDanger, OnClick: ReplaceServerCopyWithThisDevice, Disabled: !protected})),
		If(!hosted && step != cloudOff && step != cloudDone, uic.CreateElement(cloudSetupView, cloudSetupProps{Step: step, Where: where, SignIn: signin, Protect: protection, First: first})),
		If(hosted, uic.CreateElement(cloudHostedSetupView, cloudHostedSetupProps{Step: step, Protect: protection, First: first})),
		If(step == cloudDone, cloudHouseholdSummary()),
		If(step == cloudDone || (!hosted && step == cloudOff), protection),
		If(status.State == "conflict" || hasConflictBackup(workspaceID), uic.CreateElement(cloudConflictEntry, cloudConflictEntryProps{OnReview: openSyncConflict})),
		If(signedIn, uic.CreateElement(cloudDevicesView, devices)),
		If(signedIn, uic.CreateElement(cloudAccountView, cloudAccountProps{Account: signedInUserID(), Server: backendHost(pr.ServerURL), OnSignOut: auth.SignOut})),
		uic.CreateElement(cloudDisclosure, cloudDisclosureProps{Title: uistate.T("cloud1.advanced"), TestID: "cloud-advanced", Body: Fragment(
			If(!hosted, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.changeConnection"), TestID: "cloud-change-connection", OnClick: setup.Start, Disabled: setup.Busy})),
			If(status.Message != "", P(css.Class(tw.Text12, tw.TextDim), Attr("data-testid", "sync-status-detail"), status.Message)),
			If(segmentFor(pr) == "commercial", uic.CreateElement(cloudBillingView, cloudBillingProps{SignedIn: signedIn})),
		)}),
		If(signedIn || !pr.BackendDisabled, uic.CreateElement(cloudDangerView, cloudDangerProps{Household: cloudHouseholdName(), Server: backendHost(pr.ServerURL), OnPause: setup.PauseThen, OnDelete: func(done func(error)) { cloudDeleteCopy(pr, workspaceID, setup.PauseThen, done) }})),
	)
}

func cloudHouseholdName() string {
	if w, ok := loadRegistry().Active(); ok {
		return w.Name
	}
	return uistate.T("cloud1.household")
}

func cloudHouseholdSummary() uic.Node {
	counts := ""
	if a := appstate.Default; a != nil {
		counts = uistate.T("cloud1.contents", len(a.Accounts()), len(a.Transactions()), len(a.Budgets()), len(a.Artifacts()))
	}
	return Section(css.Class(cloudSectionRules()),
		H3(css.Class(tw.Text15, tw.FontSemibold), cloudHouseholdName()),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.whatSyncs")),
		P(css.Class(tw.Text12, tw.TextDim), counts),
		P(css.Class(tw.Text12, tw.TextDim), uistate.T("cloud1.sizeHint")),
	)
}
