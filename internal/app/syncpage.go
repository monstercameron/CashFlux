// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// The screens registry can't import this package (app imports screens), so the
// /sync route's view is injected at boot — mirroring settings_route.go.
func init() {
	screens.SyncView = func() uic.Node { return uic.CreateElement(SyncPage) }
}

// SyncPage is the routed /sync page body: a focused, top-level surface to connect
// a backend, toggle sync on or off, and see live sync status. It is a purpose-built
// companion to the Settings → Cloud tab (which keeps the fuller subscription /
// sign-in / devices surface): this page reuses the SAME sync engine and prefs
// (requestBackendSyncNow, the prefs atom, loadSyncStatus) rather than forking any
// logic, and links out to Cloud settings for billing and per-device management.
//
// What syncs: the whole dataset (the workspace snapshot pushed to the backend) AND
// attached artifact files (uploaded as content-addressed blobs). When a passcode
// lock is active the dataset is encrypted on this device first.
func SyncPage() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()
	// Re-render on any sync activity (push/pull bump the shared revision) so the
	// live status card reflects reality without a manual refresh.
	_ = uistate.UseDataRevision().Get()
	pr := prefsAtom.Get().Normalize()

	nav := router.UseNavigate()
	serverURL := uic.UseState(pr.ServerURL)
	serverToken := uic.UseState(pr.ServerToken)
	backendOn := uic.UseState(!pr.BackendDisabled)
	serverMode := uic.UseState(string(pr.ServerMode))

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	persist := func(p prefs.Prefs) {
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
	}

	// The connect switch: off cleanly stops every sync/AI-proxy connection even with
	// a URL/token saved, so an unreachable server never throws websocket errors the
	// user can't dismiss. On kicks an immediate sync so the user sees it work. Plain
	// funcs (not UseEvent) — ToggleRow/Segmented are their own components and own the
	// click/change hook, so these must be ordinary callbacks.
	onToggle := func(v bool) {
		backendOn.Set(v)
		p := prefsAtom.Get()
		p.BackendDisabled = !v
		persist(p)
		// Take effect immediately: restart starts the watch (and lifecycle listeners)
		// when on, or tears it down when off — no page reload required.
		restartBackendSync()
	}
	onMode := func(v string) {
		serverMode.Set(v)
		p := prefsAtom.Get()
		p.ServerMode = prefs.ServerMode(v)
		persist(p)
	}
	// OnInput/OnClick want a ui.Handler, so these are UseEvent-wrapped at stable
	// (non-loop) positions.
	onURL := uic.UseEvent(func(v string) {
		serverURL.Set(v)
		p := prefsAtom.Get()
		next := strings.TrimSpace(v)
		// Pointing at a different server (host change) signs out of the old one — a
		// token issued by one server is meaningless to another — matching the Cloud
		// settings behaviour. Editing only the path/query of the same host keeps the
		// session.
		hostChanged := backendHost(next) != backendHost(p.ServerURL)
		if hostChanged && p.ServerToken != "" {
			p.ServerToken = ""
			p.ServerCSRF = ""
			serverToken.Set("")
			setSyncStatus(syncStatus{State: "offline"})
			notify(uistate.T("settings.serverSwitched"), false)
		}
		p.ServerURL = next
		persist(p)
		// A host change points sync at a different server — restart the watch against
		// it now (rather than waiting for the old connection to drop). Same-host edits
		// (path/query) don't thrash the watch; the loop re-reads prefs on next reconnect.
		if hostChanged {
			restartBackendSync()
		}
	})
	onToken := uic.UseEvent(func(v string) {
		serverToken.Set(v)
		p := prefsAtom.Get()
		p.ServerToken = strings.TrimSpace(v)
		persist(p)
	})
	// Enabling a passcode lock is what turns on zero-knowledge (E2E) sync: the dataset is encrypted
	// on-device before upload, so the server only ever holds ciphertext. Offered right on the sync
	// page so the user can make that privacy decision here.
	onEnablePasscode := uic.UseEvent(func() { setPasscodeFlow() })
	onTest := uic.UseEvent(func() {
		testBackendConnection(serverURL.Get(), serverToken.Get(), func(discovery backendauth.Discovery) {
			discovery = discovery.Normalize()
			notify(uistate.T("settings.serverTestOK", discovery.AuthMode), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverTestFailed", strings.TrimSpace(msg)), true)
		})
	})
	onSyncNow := uic.UseEvent(func() {
		requestBackendSyncNow()
		notify(uistate.T("sync.syncingNow"), false)
	})
	onOpenSettings := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/settings")) })

	cloudSelected := prefs.ServerMode(serverMode.Get()) == prefs.ServerCloud
	status := loadSyncStatus()

	return Div(css.Class("sync-page", tw.Flex, tw.FlexCol, tw.Gap4),
		// Framing: what this page is for and exactly what leaves the device.
		P(css.Class(tw.TextDim, tw.Text14), uistate.T("sync.intro")),

		// Live status card.
		Div(css.Class("sync-status-card", tw.Flex, tw.ItemsCenter, tw.Gap3, tw.Px3, tw.Py2, tw.Rounded4, tw.Border, tw.BorderLine),
			Attr("role", "status"), Attr("data-testid", "sync-status-card"),
			ui.Icon(icon.Cloud, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.MinW0),
				Span(css.Class(tw.Text15, tw.FontSemibold), syncStatusLabel()),
				If(status.Pending > 0, Span(css.Class(tw.Text12, tw.TextFaint),
					uistate.T("sync.pendingCount", status.Pending))),
			),
		),

		// The connect toggle.
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("sync.connectToggle"), On: backendOn.Get(), OnChange: onToggle}),
		If(!backendOn.Get(), P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.offHint"))),

		// Connection form (only when on).
		If(backendOn.Get(), Fragment(
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.serverMode"),
				Options: []ui.SegOption{
					{Value: string(prefs.ServerCloud), Label: uistate.T("settings.serverModeCloud")},
					{Value: string(prefs.ServerSelfHosted), Label: uistate.T("settings.serverModeSelf")},
				},
				Selected: serverMode.Get(),
				OnSelect: onMode,
			}),
			Input(css.Class("set-input"), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")),
				Attr("data-testid", "sync-server-url"),
				Placeholder(defaultBackendURL), Value(serverURL.Get()), OnInput(onURL)),
			// Self-hosted uses a bearer token; Cloud uses OAuth sign-in (kept on the
			// Settings → Cloud tab), so the token field shows only for self-hosted.
			If(!cloudSelected, Input(css.Class("set-input"), Type("password"),
				Attr("aria-label", uistate.T("settings.backendToken")), Attr("data-testid", "sync-server-token"),
				Placeholder(uistate.T("settings.backendToken")), Value(serverToken.Get()), OnInput(onToken))),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt1),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-test"), OnClick(onTest), uistate.T("settings.testBackend")),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "sync-now"), OnClick(onSyncNow), uistate.T("settings.syncNow")),
			),
			If(cloudSelected, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.cloudSignInHint"))),
		)),

		// What syncs — the honest disclosure, always visible.
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.whatSyncs")),

		// Privacy / end-to-end encryption status — always visible so the user can make the
		// zero-knowledge decision. When a passcode lock is on, the dataset is encrypted on-device
		// before upload (server stores ciphertext only); when it's off, the server stores readable
		// JSON, and we offer the one-tap way to turn the passcode on.
		Div(css.Class("card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2),
			Span(css.Class(tw.Text12, tw.TextFaint), uistate.T("sync.encTitle")),
			If(loadAppLock().Active(), P(css.Class(tw.Text12), uistate.T("sync.encOn"))),
			If(!loadAppLock().Active(), Fragment(
				P(css.Class(tw.Text12), uistate.T("sync.encOff")),
				Div(css.Class(tw.Mt1),
					Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "sync-enable-passcode"),
						OnClick(onEnablePasscode), uistate.T("sync.encEnable"))),
			)),
		),

		// Custom Sync: sign in with a phone number instead of a server URL/token
		// (TODOS.md C420/C419). Its own self-contained component/hooks; composed
		// in here rather than folded into this page's logic.
		uic.CreateElement(CustomSyncCard),

		// Password fallback for phone sign-in (TODOS.md C422 client UI): collapsed
		// behind a small "Use a password instead" link so it reads as the escape
		// hatch, not a co-equal option next to CustomSyncCard's phone flow.
		uic.CreateElement(PasswordAuthCard),

		// Link a new device to an EXISTING account via a portal-minted pairing code
		// (TODOS.md C421 client UI). Deliberately its own card, never folded into
		// the brand-new-user phone/password flows above, since it only ever
		// resolves an account that already exists.
		uic.CreateElement(DeviceLinkCard),

		// Link out to the fuller Cloud settings (subscription, sign-in, devices).
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "sync-open-settings"),
				OnClick(onOpenSettings), uistate.T("sync.openSettings")),
			Span(css.Class(tw.Text12, tw.TextFaint), uistate.T("sync.manageMore")),
		),
	)
}
