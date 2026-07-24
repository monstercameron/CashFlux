// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
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
	screens.SyncView = func() uic.Node { return uic.CreateElement(SyncPage) }
}

// discoveryPhase tracks where SyncPage's automatic /v1/version capability
// check currently is.
type discoveryPhase string

const (
	discoveryIdle     discoveryPhase = "idle"
	discoveryChecking discoveryPhase = "checking"
	discoveryOK       discoveryPhase = "ok"
	discoveryError    discoveryPhase = "error"
)

// SyncPage is the routed /sync page body: a focused, top-level surface to connect
// a backend, toggle sync on or off, and see live sync status. It is a purpose-built
// companion to the Settings → Cloud tab (which keeps the fuller subscription /
// devices surface): this page reuses the SAME sync engine and prefs
// (requestBackendSyncNow, the prefs atom, loadSyncStatus) rather than forking any
// logic, and links out to Cloud settings for billing and per-device management.
//
// What syncs: the whole dataset (the workspace snapshot pushed to the backend) AND
// attached artifact files (uploaded as content-addressed blobs). When a passcode
// lock is active the dataset is encrypted on this device first.
//
// Sign-in method is chosen by capability, not by a manually-picked mode
// (2026-07-23 redesign): the page asks the connected server what it actually
// supports (CustomAuthEnabled → phone/password/pairing; AuthProviders → OAuth;
// neither → a fixed access token) and shows exactly that, for any of the three
// real modalities — CashFlux Cloud, a self-hosted server, or someone else's
// server — instead of stacking every sign-in door on screen at once regardless
// of whether the connected backend actually offers it.
func SyncPage() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()
	// Re-render on any sync activity (push/pull bump the shared revision) so the
	// live status card reflects reality without a manual refresh.
	_ = uistate.UseDataRevision().Get()
	pr := prefsAtom.Get().Normalize()

	serverURL := uic.UseState(pr.ServerURL)
	serverToken := uic.UseState(pr.ServerToken)
	backendOn := uic.UseState(!pr.BackendDisabled)

	discovery := uic.UseState(backendauth.Discovery{})
	discoveryState := uic.UseState(discoveryIdle)
	discoveryMsg := uic.UseState("")
	advancedTokenOpen := uic.UseState(false)
	// manualAddressOpen gates the server-address field itself: false means
	// "still trying (or succeeded at) auto-detecting a same-origin backend,
	// nothing to type" — true means the user needs to (or chose to) enter an
	// address by hand. Starts true for anyone who already has a REAL configured
	// address (a returning self-host/Cloud user) so their existing setup is
	// never silently overridden by the same-origin probe. Compared against
	// prefs.DefaultServerURL, not "" — prefs.Default() itself (loadPrefs, on a
	// never-persisted user) already fills ServerURL with that placeholder, so
	// checking for blank never actually distinguishes a fresh user; the
	// placeholder is a local-loopback convenience default, not a real choice,
	// so it's safe to try same-origin first for it too.
	manualAddressOpen := uic.UseState(strings.TrimSpace(prefsAtom.Get().ServerURL) != prefs.DefaultServerURL)

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	persist := func(p prefs.Prefs) {
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
	}

	// runDiscovery asks the currently-typed server what it supports. Called on
	// mount and whenever the server address's HOST actually changes (not on
	// every keystroke — see onURL below) so it never spams the network while
	// someone is still typing.
	runDiscovery := func() {
		url := strings.TrimSpace(serverURL.Get())
		if url == "" {
			discoveryState.Set(discoveryIdle)
			return
		}
		discoveryState.Set(discoveryChecking)
		testBackendConnection(url, serverToken.Get(), func(d backendauth.Discovery) {
			discovery.Set(d)
			discoveryState.Set(discoveryOK)
		}, func(msg string) {
			discoveryMsg.Set(strings.TrimSpace(msg))
			discoveryState.Set(discoveryError)
		})
	}

	// probeSameOrigin tries this document's own origin as the backend address
	// BEFORE ever asking the user to type anything — the zero-config path for
	// "this page is served by the same server that runs the sync bridge"
	// (e.g. CashFlux mounted at /budget/ on a site whose backend also serves
	// /grpc). Only attempted for someone with no address configured yet;
	// success persists that origin as ServerURL so CustomSyncCard/
	// PasswordAuthCard/DeviceLinkCard (which read prefs directly) pick it up
	// too. Failure just falls through to the manual address field — the
	// normal, non-embedded desktop-app case, where no backend lives at the
	// page's own origin at all.
	probeSameOrigin := func() {
		origin := currentPageOrigin()
		discoveryState.Set(discoveryChecking)
		testBackendConnection(origin, "", func(d backendauth.Discovery) {
			serverURL.Set(origin)
			p := prefsAtom.Get()
			p.ServerURL = origin
			persist(p)
			discovery.Set(d)
			discoveryState.Set(discoveryOK)
		}, func(string) {
			manualAddressOpen.Set(true)
			discoveryState.Set(discoveryIdle)
		})
	}

	uic.UseEffect(func() func() {
		if !backendOn.Get() {
			return nil
		}
		if manualAddressOpen.Get() {
			runDiscovery()
		} else {
			probeSameOrigin()
		}
		return nil
	}, "sync-discovery-mount")

	onUseDifferentAddress := uic.UseEvent(func() { manualAddressOpen.Set(true) })

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
		if v {
			if manualAddressOpen.Get() {
				runDiscovery()
			} else {
				probeSameOrigin()
			}
		}
	}
	// OnInput/OnClick want a ui.Handler, so these are UseEvent-wrapped at stable
	// (non-loop) positions.
	onURL := uic.UseEvent(func(v string) {
		serverURL.Set(v)
		p := prefsAtom.Get()
		next := strings.TrimSpace(v)
		// Pointing at a different server (host change) signs out of the old one — a
		// token issued by one server is meaningless to another — matching the Cloud
		// settings behaviour, and re-checks what the new host actually supports.
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
		if hostChanged {
			restartBackendSync()
			runDiscovery()
		}
	})
	onToken := uic.UseEvent(func(v string) {
		serverToken.Set(v)
		p := prefsAtom.Get()
		p.ServerToken = strings.TrimSpace(v)
		persist(p)
	})
	// saveOAuthSession lands a Google/GitHub session the same way Settings' Cloud
	// tab does (internal/app/settings.go's own saveOAuthSession) — kept as a
	// separate closure rather than shared, since the two pages hold distinct
	// serverURL/serverToken UseState instances.
	saveOAuthSession := func(token, csrf, userID string) {
		p := prefsAtom.Get()
		p.ServerToken = token
		p.ServerCSRF = csrf
		p.ServerURL = serverURL.Get()
		persist(p)
		serverToken.Set(token)
		if strings.TrimSpace(userID) == "" {
			notify(uistate.T("settings.oauthSignedIn"), false)
		} else {
			notify(uistate.T("settings.oauthSignedInAs", userID), false)
		}
		restartBackendSync()
	}
	onSignInGoogle := uic.UseEvent(func() {
		startOAuthLogin(serverURL.Get(), "google", saveOAuthSession, func(msg string) { notify(msg, true) })
	})
	onSignInGitHub := uic.UseEvent(func() {
		startOAuthLogin(serverURL.Get(), "github", saveOAuthSession, func(msg string) { notify(msg, true) })
	})
	onEnablePasscode := uic.UseEvent(func() { setPasscodeFlow() })
	onToggleAdvancedToken := uic.UseEvent(func() { advancedTokenOpen.Set(!advancedTokenOpen.Get()) })
	onTest := uic.UseEvent(func() {
		testBackendConnection(serverURL.Get(), serverToken.Get(), func(d backendauth.Discovery) {
			discovery.Set(d)
			discoveryState.Set(discoveryOK)
			notify(uistate.T("settings.serverTestOK", d.AuthMode), false)
		}, func(msg string) {
			discoveryMsg.Set(strings.TrimSpace(msg))
			discoveryState.Set(discoveryError)
			notify(uistate.T("settings.serverTestFailed", strings.TrimSpace(msg)), true)
		})
	})
	onSyncNow := uic.UseEvent(func() {
		requestBackendSyncNow()
		notify(uistate.T("sync.syncingNow"), false)
	})
	onOpenSettings := uic.UseEvent(func() { uistate.OpenGlobalSettingsAt("cloud") })

	status := loadSyncStatus()
	d := discovery.Get()
	phase := discoveryState.Get()
	showPhone := phase == discoveryOK && d.CustomAuthEnabled
	showOAuth := phase == discoveryOK && len(d.AuthProviders) > 0
	// The token field is the primary (only) option once discovery resolves with
	// neither phone nor OAuth support, a safe fallback while discovery is still
	// checking/erroring (an address might still work even if we couldn't
	// confirm capabilities), and otherwise a quiet opt-in "advanced" disclosure.
	tokenPrimary := phase != discoveryOK || (!showPhone && !showOAuth)
	showTokenField := tokenPrimary || advancedTokenOpen.Get()

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

		// Connection form (only when on): one address field for all three
		// modalities (CashFlux Cloud, your own server, someone else's server) —
		// what renders below it is chosen by what that address reports
		// supporting, not by a manually-picked mode.
		If(backendOn.Get(), Fragment(
			// Zero-config path: a same-origin backend was found automatically —
			// no address field at all, just a quiet way to override it.
			If(!manualAddressOpen.Get() && phase == discoveryOK, Button(css.Class("btn-link", tw.Text12, tw.TextDim), Type("button"),
				Attr("data-testid", "sync-use-different-address"), OnClick(onUseDifferentAddress), uistate.T("sync.useDifferentAddress"))),

			If(manualAddressOpen.Get(), Fragment(
				P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.serverAddressIntro")),
				Input(css.Class("set-input"), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")),
					Attr("data-testid", "sync-server-url"),
					Placeholder(defaultBackendURL), Value(serverURL.Get()), OnInput(onURL)),
			)),

			If(phase == discoveryChecking, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "sync-discovery-checking"), uistate.T("sync.discoveryChecking"))),
			If(phase == discoveryOK, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "sync-discovery-ok"), uistate.T("sync.discoveryOK"))),
			If(phase == discoveryError, P(css.Class(tw.Text12, tw.TextFaint), Attr("data-testid", "sync-discovery-error"), uistate.T("settings.serverTestFailed", discoveryMsg.Get()))),

			// Phone/password/pairing — only when this server actually has AuthService.
			If(showPhone, Fragment(
				uic.CreateElement(CustomSyncCard),
				uic.CreateElement(PasswordAuthCard),
				uic.CreateElement(DeviceLinkCard),
			)),

			// OAuth — only for the providers this server actually reports.
			If(showOAuth, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt1),
				If(containsString(d.AuthProviders, "google"), Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-oauth-google"), OnClick(onSignInGoogle), uistate.T("settings.signInGoogle"))),
				If(containsString(d.AuthProviders, "github"), Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-oauth-github"), OnClick(onSignInGitHub), uistate.T("settings.signInGitHub"))),
			)),

			// Fixed access token — primary when nothing else is available (or
			// discovery hasn't resolved yet), otherwise a quiet "advanced" opt-in.
			If(tokenPrimary && phase == discoveryOK, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.tokenFieldPrimary"))),
			If(!tokenPrimary && !advancedTokenOpen.Get(), Button(css.Class("btn-link", tw.Text12, tw.TextDim), Type("button"),
				Attr("data-testid", "sync-advanced-token-toggle"), OnClick(onToggleAdvancedToken), uistate.T("sync.advancedTokenToggle"))),
			If(showTokenField, Fragment(
				Input(css.Class("set-input"), Type("password"),
					Attr("aria-label", uistate.T("settings.backendToken")), Attr("data-testid", "sync-server-token"),
					Placeholder(uistate.T("settings.backendToken")), Value(serverToken.Get()), OnInput(onToken)),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt1),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-test"), OnClick(onTest), uistate.T("settings.testBackend")),
					Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "sync-now"), OnClick(onSyncNow), uistate.T("settings.syncNow")),
				),
			)),
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

		// Link out to the fuller Cloud settings (subscription, devices) — sign-in
		// itself now happens inline above, so this is purely account management.
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "sync-open-settings"),
				OnClick(onOpenSettings), uistate.T("sync.openSettings")),
			Span(css.Class(tw.Text12, tw.TextFaint), uistate.T("sync.manageMore")),
		),
	)
}
