// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
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

// statusLabelColor returns the alarm color for a genuine sync error, or nil
// (silently ignored by css.Class) for every other status — a status that is
// merely offline/syncing/conflicted doesn't warrant the same visual alarm as
// one the app has confirmed actually failed.
func statusLabelColor(state string) any {
	if state == "error" {
		return tw.TextDanger
	}
	return nil
}

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

// discoveryPhase tracks where the Cloud tab's automatic /v1/version capability
// check currently is. Only meaningful for the Local/Remote segments — the
// Commercial segment (CashFlux Cloud) skips discovery entirely.
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

// CloudConnectionPane is the Settings → Cloud tab body: the single place to
// connect a backend, toggle sync on or off, manage sign-in, and (for CashFlux
// Cloud) subscribe. It replaces two previously-drifted implementations — the
// old routed /sync page and the old Cloud tab's raw-bearer-token-only UI
// (2026-07-24 unification) — with one capability-aware surface, self-contained
// like PasswordAuthCard/DeviceLinkCard rather than prop-driven, since every
// field here is this pane's own concern.
//
// What syncs: the whole dataset (the workspace snapshot pushed to the backend) AND
// attached artifact files (uploaded as content-addressed blobs). When a passcode
// lock is active the dataset is encrypted on this device first.
//
// Three segments, not a single address field:
//   - Local: same-origin auto-probe by default (your own infrastructure at the
//     obvious address), with a manual fallback for unusual URLs (subdomains).
//   - Remote: always a manually-typed address, with an explicit trust
//     disclosure — this may be someone else's server.
//   - Commercial: the CashFlux Cloud subscription. Skips discovery entirely
//     (goes straight to OAuth/token sign-in + billing) since a paid backend's
//     capabilities are a known quantity, not something to probe for.
//
// Within Local/Remote, sign-in method is chosen by capability, not a manually
// picked mode: the page asks the connected server what it actually supports
// (CustomAuthEnabled → password/pairing; AuthProviders → OAuth; neither → a
// fixed access token) and shows exactly that.
func CloudConnectionPane() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()
	dataRev := uistate.UseDataRevision()
	// Re-render on any sync activity (push/pull bump the shared revision) so the
	// live status card/devices list/conflict backup reflect reality without a
	// manual refresh.
	_ = dataRev.Get()
	pr := prefsAtom.Get().Normalize()

	segment := uic.UseState(segmentFor(pr))
	serverURL := uic.UseState(pr.ServerURL)
	serverToken := uic.UseState(pr.ServerToken)
	backendOn := uic.UseState(!pr.BackendDisabled)

	discovery := uic.UseState(backendauth.Discovery{})
	discoveryState := uic.UseState(discoveryIdle)
	discoveryMsg := uic.UseState("")
	advancedTokenOpen := uic.UseState(false)
	billingInterval := uic.UseState("annual")
	billingProvider := uic.UseState("stripe")
	keySet := uic.UseState(lsGet("cashflux:cloud-ai-key-set") != "")
	// manualAddressOpen gates the server-address field for the Local segment:
	// false means "still trying (or succeeded at) auto-detecting a same-origin
	// backend, nothing to type" — true means the user needs to (or chose to)
	// enter an address by hand. The Remote segment always starts (and stays)
	// manual — there is no same-origin assumption for someone else's server.
	// Local starts true for anyone who already has a REAL configured address (a
	// returning self-host user) so their existing setup is never silently
	// overridden by the same-origin probe; compared against
	// prefs.DefaultServerURL, not "", since prefs.Default() itself already
	// fills ServerURL with that placeholder for a never-persisted user.
	manualAddressOpen := uic.UseState(segment.Get() == "remote" ||
		strings.TrimSpace(pr.ServerURL) != prefs.DefaultServerURL)

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	bump := func() { dataRev.Update(func(n int) int { return n + 1 }) }
	persist := func(p prefs.Prefs) {
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
	}

	// runDiscovery asks the currently-typed server what it supports. Called on
	// mount and whenever the server address's HOST actually changes (not on
	// every keystroke — see onURL below) so it never spams the network while
	// someone is still typing. Never called for the Commercial segment.
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
	// "this page is served by the same server that runs the sync bridge" (e.g.
	// CashFlux mounted at /budget/ on a site whose backend also serves /grpc).
	// Local segment only; failure just falls through to the manual address
	// field — the normal, non-embedded desktop-app case.
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

	connect := func() {
		if !backendOn.Get() || segment.Get() == "commercial" {
			return
		}
		if manualAddressOpen.Get() {
			runDiscovery()
		} else {
			probeSameOrigin()
		}
	}

	uic.UseEffect(func() func() {
		connect()
		return nil
	}, "cloud-tab-discovery-mount")

	onUseDifferentAddress := uic.UseEvent(func() { manualAddressOpen.Set(true) })

	// onSelectSegment switches between Local/Remote/Commercial. Each is a
	// distinct trust/discovery posture, so switching drops any in-flight
	// discovery result from the PREVIOUS segment's server rather than
	// reinterpreting it under the new one.
	onSelectSegment := func(v string) {
		segment.Set(v)
		p := prefsAtom.Get()
		switch v {
		case "commercial":
			p.ServerMode = prefs.ServerCloud
		case "remote":
			p.ServerMode = prefs.ServerSelfHosted
			p.ConnectionSegment = prefs.ConnectionRemote
		default: // "local"
			p.ServerMode = prefs.ServerSelfHosted
			p.ConnectionSegment = prefs.ConnectionLocal
		}
		persist(p)
		discovery.Set(backendauth.Discovery{})
		discoveryState.Set(discoveryIdle)
		manualAddressOpen.Set(v == "remote")
		restartBackendSync()
		connect()
	}

	// The connect switch: off cleanly stops every sync/AI-proxy connection even with
	// a URL/token saved, so an unreachable server never throws websocket errors the
	// user can't dismiss. On kicks an immediate connect so the user sees it work.
	onToggle := func(v bool) {
		backendOn.Set(v)
		p := prefsAtom.Get()
		p.BackendDisabled = !v
		persist(p)
		restartBackendSync()
		if v {
			connect()
		}
	}
	onURL := uic.UseEvent(func(v string) {
		serverURL.Set(v)
		p := prefsAtom.Get()
		next := strings.TrimSpace(v)
		// Pointing at a different server (host change) signs out of the old one — a
		// token issued by one server is meaningless to another — and re-checks what
		// the new host actually supports.
		hostChanged := backendHost(next) != backendHost(p.ServerURL)
		if hostChanged && p.ServerToken != "" {
			p.ServerToken = ""
			p.ServerCSRF = ""
			serverToken.Set("")
			lsSet("cashflux:cloud-ai-key-set", "")
			keySet.Set(false)
			setSyncStatus(syncStatus{State: "offline"})
			notify(uistate.T("settings.serverSwitched"), false)
		}
		p.ServerURL = next
		persist(p)
		if hostChanged {
			restartBackendSync()
			if segment.Get() != "commercial" {
				runDiscovery()
			}
		}
	})
	onToken := uic.UseEvent(func(v string) {
		serverToken.Set(v)
		p := prefsAtom.Get()
		p.ServerToken = strings.TrimSpace(v)
		persist(p)
	})
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
	onSignOut := uic.UseEvent(func() {
		p := prefsAtom.Get()
		signOutBackendOAuth(serverURL.Get(), p.ServerToken, p.ServerCSRF, func() {
			p.ServerToken = ""
			p.ServerCSRF = ""
			persist(p)
			serverToken.Set("")
			notify(uistate.T("settings.oauthSignedOut"), false)
		})
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
	uploadKey := uic.UseEvent(func() {
		key := ""
		if a := appstate.Default; a != nil {
			key = a.Settings().OpenAIKey
		}
		uploadOpenAIKeyToBackend(serverURL.Get(), serverToken.Get(), key, func() {
			lsSet("cashflux:cloud-ai-key-set", "1")
			keySet.Set(true)
			notify(uistate.T("settings.serverKeyStored"), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverKeyFailed", strings.TrimSpace(msg)), true)
		})
	})
	removeKey := uic.UseEvent(func() {
		removeOpenAIKeyFromBackend(serverURL.Get(), serverToken.Get(), func() {
			lsSet("cashflux:cloud-ai-key-set", "")
			keySet.Set(false)
			notify(uistate.T("settings.serverKeyRemoved"), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverKeyFailed", strings.TrimSpace(msg)), true)
		})
	})
	startCheckout := uic.UseEvent(func() {
		startBillingCheckout(serverURL.Get(), serverToken.Get(), billingInterval.Get(), billingProvider.Get(), func(msg string) {
			notify(uistate.T("settings.billingFailed", strings.TrimSpace(msg)), true)
		})
	})
	openPortal := uic.UseEvent(func() {
		openBillingPortal(serverURL.Get(), serverToken.Get(), func(msg string) {
			notify(uistate.T("settings.billingFailed", strings.TrimSpace(msg)), true)
		})
	})
	// C309: restore / discard a local edit that lost an LWW conflict.
	activeWsID := loadRegistry().ActiveID
	onRestoreConflict := uic.UseEvent(func() {
		if restoreConflictBackup(activeWsID) {
			notify(uistate.T("sync.conflictRestored"), false)
		}
		bump()
	})
	onDiscardConflict := uic.UseEvent(func() {
		clearConflictBackup(activeWsID)
		notify(uistate.T("sync.conflictDiscarded"), false)
		bump()
	})

	status := loadSyncStatus()
	d := discovery.Get()
	phase := discoveryState.Get()
	seg := segment.Get()
	commercial := seg == "commercial"
	showPassword := !commercial && phase == discoveryOK && d.CustomAuthEnabled
	// Commercial always offers OAuth — a paid backend's capabilities are a known
	// quantity, not something to probe for — so discovery is never even run.
	showOAuth := commercial || (phase == discoveryOK && len(d.AuthProviders) > 0)
	tokenPrimary := commercial || phase != discoveryOK || (!showPassword && !showOAuth)
	showTokenField := tokenPrimary || advancedTokenOpen.Get()
	cloudPrice := uistate.T("settings.cloudPriceAnnual")
	if billingInterval.Get() == "monthly" {
		cloudPrice = uistate.T("settings.cloudPriceMonthly")
	}

	tokenField := Fragment(
		Input(css.Class("set-input"), Type("password"),
			Attr("aria-label", uistate.T("settings.backendToken")), Attr("data-testid", "sync-server-token"),
			Placeholder(uistate.T("settings.backendToken")), Value(serverToken.Get()), OnInput(onToken)),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt1),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-test"), OnClick(onTest), uistate.T("settings.testBackend")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "sync-now"), OnClick(onSyncNow), uistate.T("settings.syncNow")),
		),
	)

	return Div(css.Class("cloud-tab", tw.Flex, tw.FlexCol, tw.Gap4),
		H4(css.Class("set-label"), uistate.T("settings.backendTitle")),
		P(css.Class(tw.TextDim, tw.Text14), uistate.T("settings.cloudSectionIntro")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.cloudDataDisclosure")),

		// Live status card.
		Div(css.Class("sync-status-card", tw.Flex, tw.ItemsCenter, tw.Gap3, tw.Px3, tw.Py2, tw.Rounded4, tw.Border, tw.BorderLine),
			Attr("role", "status"), Attr("data-testid", "sync-status-card"),
			ui.Icon(icon.Cloud, css.Class(tw.W5, tw.H5, tw.ShrinkO, IfElseValue(status.State == "error", tw.TextDanger, tw.TextDim))),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.MinW0),
				Span(css.Class(tw.Text15, tw.FontSemibold, statusLabelColor(status.State)), syncStatusLabel()),
				If(status.Pending > 0, Span(css.Class(tw.Text12, tw.TextFaint),
					uistate.T("sync.pendingCount", status.Pending))),
				If(status.Message != "", Span(css.Class(tw.Text12, IfElseValue(status.State == "error", tw.TextDanger, tw.TextFaint)), Attr("data-testid", "sync-status-detail"),
					uistate.T("sync.statusDetail", status.Message))),
			),
		),

		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("sync.connectToggle"), On: backendOn.Get(), OnChange: onToggle}),
		If(!backendOn.Get(), P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.backendOffHint"))),

		If(backendOn.Get(), Fragment(
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("sync.segmentLabel"),
				Options: []ui.SegOption{
					{Value: "local", Label: uistate.T("sync.segmentLocal")},
					{Value: "remote", Label: uistate.T("sync.segmentRemote")},
					{Value: "commercial", Label: uistate.T("sync.segmentCommercial")},
				},
				Selected: seg,
				OnSelect: onSelectSegment,
			}),
			If(seg == "local", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("sync.segmentLocalHint"))),
			If(seg == "remote", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("sync.segmentRemoteHint"))),
			If(commercial, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("sync.segmentCommercialHint"))),

			// Local: zero-config path when a same-origin backend was found — no
			// address field at all, just a quiet way to override it.
			If(seg == "local" && !manualAddressOpen.Get() && phase == discoveryOK,
				Button(css.Class("btn-link", tw.Text12, tw.TextDim), Type("button"),
					Attr("data-testid", "sync-use-different-address"), OnClick(onUseDifferentAddress), uistate.T("sync.useDifferentAddress"))),

			// Local (manual fallback) and Remote (always manual) both need the
			// address field; Commercial gets its own below.
			If(seg != "commercial" && manualAddressOpen.Get(), Fragment(
				If(seg == "remote", P(css.Class(tw.Text12, tw.TextDanger), uistate.T("sync.remoteTrustDisclosure"))),
				If(seg == "local", P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.serverAddressIntro"))),
				Input(css.Class("set-input"), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")),
					Attr("data-testid", "sync-server-url"),
					Placeholder(defaultBackendURL), Value(serverURL.Get()), OnInput(onURL)),
			)),
			If(seg != "commercial", Fragment(
				If(phase == discoveryChecking, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "sync-discovery-checking"), uistate.T("sync.discoveryChecking"))),
				If(phase == discoveryOK, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "sync-discovery-ok"), uistate.T("sync.discoveryOK"))),
				If(phase == discoveryError, P(css.Class(tw.Text12, tw.TextFaint), Attr("data-testid", "sync-discovery-error"), uistate.T("settings.serverTestFailed", discoveryMsg.Get()))),
			)),

			// Commercial: a fixed-shape backend — address field (defaults to
			// whatever's saved, since CashFlux Cloud isn't a single hardcoded
			// domain yet), then straight to sign-in, no capability probing.
			If(commercial, Input(css.Class("set-input"), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")),
				Attr("data-testid", "sync-server-url"),
				Placeholder(defaultBackendURL), Value(serverURL.Get()), OnInput(onURL))),
			If(commercial && strings.TrimSpace(serverToken.Get()) == "",
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudPricingTeaser", cloudPrice))),

			// Exactly one primary sign-in surface, chosen by what the server
			// actually reports supporting (or, for Commercial, always OAuth).
			If(showPassword, uic.CreateElement(PasswordAuthCard)),
			If(showOAuth, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt1),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-oauth-google"), OnClick(onSignInGoogle), uistate.T("settings.signInGoogle")),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "sync-oauth-github"), OnClick(onSignInGitHub), uistate.T("settings.signInGitHub")),
			)),
			If(tokenPrimary && !commercial && phase == discoveryOK, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.tokenFieldPrimary"))),
			If(tokenPrimary && showTokenField, tokenField),

			If(showPassword || !tokenPrimary, Div(css.Class(tw.Mt2, tw.Flex, tw.FlexCol, tw.Gap1),
				Span(css.Class(tw.Text11, tw.Uppercase, tw.Tracking008, tw.TextFaint), uistate.T("sync.otherWaysHeading")),
				If(showPassword, uic.CreateElement(DeviceLinkCard)),
				If(!tokenPrimary, Fragment(
					If(!advancedTokenOpen.Get(), Div(Button(css.Class("btn-link", tw.Text12, tw.TextDim), Type("button"),
						Attr("data-testid", "sync-advanced-token-toggle"), OnClick(onToggleAdvancedToken), uistate.T("sync.advancedTokenToggle")))),
					If(advancedTokenOpen.Get(), tokenField),
				)),
			)),

			// Sign out / clear-invalid-token — "Sign out" implies an active session,
			// misleading once the server has explicitly rejected the saved token.
			If(strings.TrimSpace(serverToken.Get()) != "" && !status.AuthFailed,
				Button(css.Class("btn", tw.Mt1), Type("button"), OnClick(onSignOut), uistate.T("settings.signOut"))),
			If(strings.TrimSpace(serverToken.Get()) != "" && status.AuthFailed,
				Button(css.Class("btn", tw.Mt1), Type("button"), Attr("data-testid", "settings-clear-invalid-token"), OnClick(onSignOut), uistate.T("settings.clearInvalidToken"))),

			If(seg != "commercial", Div(css.Class(tw.Mt1),
				A(css.Class("btn"), Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/docs/SELF_HOSTING.md"), Attr("target", "_blank"), Attr("rel", "noreferrer"), uistate.T("settings.deploySelfHost")),
			)),
		)),

		// C309: recoverable conflict backup — when a local edit lost an LWW
		// conflict (server had newer changes), offer to restore the saved local
		// copy or discard the backup, so the change is never silently lost.
		If(hasConflictBackup(activeWsID), Div(css.Class("conflict-restore", tw.Flex, tw.FlexCol, tw.Gap1, tw.Mt1, tw.Px3, tw.Py2, tw.Rounded4, tw.BorderL),
			Attr("role", "status"), Attr("data-testid", "sync-conflict-restore"),
			P(css.Class(tw.Text12, tw.TextDim), uistate.T("sync.restoreConflictHint")),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt1),
				Button(css.Class("btn", "btn-sm", "btn-primary"), Type("button"), OnClick(onRestoreConflict), uistate.T("sync.restoreConflict")),
				Button(css.Class("btn", "btn-sm"), Type("button"), OnClick(onDiscardConflict), uistate.T("sync.discardConflict")),
			),
		)),

		// Cloud AI-key status: "Key set" + Remove/Upload, shown once authenticated (§7.11).
		If(strings.TrimSpace(serverToken.Get()) != "", Fragment(
			If(keySet.Get(), Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
				Span(css.Class(tw.Text12, tw.TextDim), uistate.T("settings.serverKeySet")),
				Button(css.Class("btn", "btn-sm", "btn-del"), Type("button"), OnClick(removeKey), uistate.T("settings.removeKey")),
			)),
			If(!keySet.Get(), Button(css.Class("btn", "btn-sm", tw.Mt1), Type("button"), OnClick(uploadKey), uistate.T("settings.uploadKey"))),
			// Signed-in devices list + per-device revoke (§7.11).
			uic.CreateElement(DevicesList),
		)),

		// Commercial-only: subscription surface.
		If(commercial, Fragment(
			If(strings.TrimSpace(serverToken.Get()) != "", Fragment(
				H4(css.Class("set-label"), uistate.T("settings.manageSubTitle")),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.manageSubHint")),
				Button(css.Class("btn", tw.Mt045), Type("button"),
					Attr("data-testid", "manage-subscription"),
					OnClick(openPortal),
					uistate.T("settings.manageSub"),
				),
			)),
			If(strings.TrimSpace(serverToken.Get()) == "", Fragment(
				H4(css.Class("set-label"), uistate.T("settings.cloudPlanTitle")),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudPlanNote")),
				Div(css.Class(tw.Text18, tw.FontSemibold, tw.Mt045), cloudPrice),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrialNote")),
				ui.Segmented(ui.SegmentedProps{
					Label: uistate.T("settings.cloudPlanBilling"),
					Options: []ui.SegOption{
						{Value: "annual", Label: uistate.T("settings.cloudPlanAnnual")},
						{Value: "monthly", Label: uistate.T("settings.cloudPlanMonthly")},
					},
					Selected: billingInterval.Get(),
					OnSelect: func(v string) { billingInterval.Set(v) },
				}),
				ui.Segmented(ui.SegmentedProps{
					Label: uistate.T("settings.cloudPayWith"),
					Options: []ui.SegOption{
						{Value: "stripe", Label: uistate.T("settings.cloudPayStripe")},
						{Value: "paypal", Label: uistate.T("settings.cloudPayPayPal")},
					},
					Selected: billingProvider.Get(),
					OnSelect: func(v string) { billingProvider.Set(v) },
				}),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt045),
					Button(css.Class("btn btn-primary"), Type("button"), OnClick(startCheckout), uistate.T("settings.cloudSubscribe")),
				),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrustLine")),
			)),
		)),

		// What syncs — the honest disclosure, always visible.
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("sync.whatSyncs")),

		// Privacy / end-to-end encryption status — always visible so the user can make the
		// zero-knowledge decision.
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
	)
}
