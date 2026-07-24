// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app wires routing and the application shell together and mounts the
// CashFlux SPA into the host page. Screen content lives in internal/screens.
package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/styles"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
	"github.com/monstercameron/GoWebComponents/v4/utils"
)

// liveCustomPageSlug reads the current custom-page slug from location.pathname at
// render time, rather than capturing it in the route's View closure. This is the
// fix for navigating *between* two custom pages: every "/p/:slug" View closure is
// created at the same source line, so they share a function code-pointer and the
// reconciler treats them as the same component — reusing the FIRST one, which had
// the old slug captured. Reading the slug live makes the closure body identical
// for every page, so it always renders the page the URL currently points at.
func liveCustomPageSlug() string {
	loc := js.Global().Get("location")
	if !loc.Truthy() {
		return ""
	}
	logical := uistate.LogicalPath(loc.Get("pathname").String()) // strip any base prefix
	const pfx = "/p/"
	if !strings.HasPrefix(logical, pfx) {
		return ""
	}
	s := logical[len(pfx):]
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

// liveSettingsTab reads the current settings tab from location.pathname at
// render time, the same live-read pattern liveCustomPageSlug uses and for the
// same reason: "/settings/:tab" is one registered route pattern shared by
// every tab, so its View closure is the same function code-pointer for all of
// them — the reconciler would otherwise treat navigating between two tabs as
// "the same component" and never see the new value if it were captured once.
// Returns "" for the bare "/settings" path (no tab segment) so the caller can
// tell "redirect to a default tab" apart from "show tab X".
func liveSettingsTab() string {
	loc := js.Global().Get("location")
	if !loc.Truthy() {
		return ""
	}
	logical := uistate.LogicalPath(loc.Get("pathname").String())
	const pfx = "/settings/"
	if !strings.HasPrefix(logical, pfx) {
		return ""
	}
	s := logical[len(pfx):]
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

// Run initializes app state, builds the router, registers every screen wrapped
// in the shell, mounts the app, and blocks the wasm runtime so the page stays
// interactive.
func Run() {
	utils.DisableAllDebug()

	// Install the app design system (the former index.html <style> blocks, now authored
	// as type-safe Go in internal/styles) into <head> before anything renders, so it is
	// present on first paint and registers ahead of the css utility engine's <style>
	// (preserving cascade order).
	styles.Register()

	// Open the IndexedDB-backed storage primitive and migrate any legacy localStorage
	// data into it FIRST — before anything reads persisted state. After this the app
	// depends on no localStorage at all; every Get/Set/Remove routes through SQLite
	// (the dataset) or this store (the dataset blob + bootstrap keys).
	browserstore.Init()
	browserstore.RegisterJSBridge() // let vendored JS (music player) persist via IndexedDB too

	// Start with an empty in-memory store, then load the user's saved dataset from
	// localStorage (or seed the sample on first run). Logs (os.Stderr) surface in
	// the browser console.
	if err := appstate.Init(nil, false); err != nil {
		panic(err)
	}
	// Initialize the workspace registry (migrates an existing single dataset into a
	// "Default" workspace). The active workspace's data already lives in the
	// canonical localStorage keys, so hydrateDataset below loads it as usual.
	ensureWorkspaceRegistry()
	// Honor the startup-workspace preference before hydrating: if a workspace is
	// pinned, swap its context into the canonical keys so the first paint is the
	// workspace the user chose to open with (no reload needed pre-mount).
	applyStartupWorkspace()
	// Open the IndexedDB artifact store so binary bytes are kept out of the main
	// localStorage JSON blob. This must run before hydrateDataset so the blob store
	// is ready when artifact bytes are migrated on load.
	initBlobStore()
	hydrateDataset()
	initUndo() // capture the baseline undo snapshot after hydration (C78)
	if appstate.Default != nil {
		appstate.Default.LoadAuditIntoFeed() // restore persisted audit history into the Activity feed (C78)
	}
	// Seed this device's music resume point from the dataset (e.g. a just-imported
	// workspace) BEFORE mounting, so the player reads the restored point on init.
	seedMusicFromDataset()
	// First-visit-only heads-up that background music defaults on (task #26).
	noticeMusicDefaultOnce()
	hydrateAIKey()
	// Fold any legacy standalone language selection into the loaded dataset (the single
	// source of truth). Only safe when the dataset is actually loaded — for an encrypted
	// dataset still awaiting unlock, defer to hydrateFromPasscode so a later ImportJSON
	// can't clobber the migrated value.
	if pendingEnvelopeRaw == "" {
		uistate.MigrateLegacyLanguage()
	}
	// Populate the lock-screen "quote of the day" cache on boot when unlocked (or
	// when a remembered on-device key is available while locked), so the lock screen
	// shows a fresh AI quote next time rather than the static fallback.
	refreshDailyLockQuote()
	// Re-apply any standing recurring budget covers whose period has rolled over.
	applyRecurringCovers()

	// Apply saved appearance preferences (theme/accent/density) before mounting,
	// so the first paint matches the user's choice instead of flashing defaults.
	uistate.ApplyPrefs(uistate.LoadPrefs())
	// Then apply the design-token theme (B20) on top: it sets the full token set
	// (surfaces, border, text, accent, radius, fonts, scale) as CSS custom
	// properties. With no saved custom theme this is migrated from the same prefs,
	// so it reproduces the default appearance until the user edits a token.
	uistate.ApplyTheme(uistate.LoadTheme())
	// Register any uploaded custom fonts (@font-face) so a theme that selects one
	// can use it from the first paint.
	uistate.ApplyFonts(uistate.LoadFonts())
	// Reflect the saved dashboard banner (gradient or uploaded image).
	uistate.ApplyBanner(uistate.LoadBanner())

	// Derive the URL sub-path the app is served under (e.g. "/CashFlux" on a
	// GitHub Pages project site) from the <base href> index.html set, so routes
	// register and navigate with that prefix (B30). At the server root the prefix
	// is "" and every RoutePath call is a no-op.
	uistate.InitRouteBase()

	r := router.NewHistoryRouter(router.RouterOptions{DefaultRoute: uistate.RoutePath("/")})
	// Register every screen at its (base-prefixed) path, each wrapped in the Shell
	// chrome. This is deliberately FLAT per-route registration: the history router
	// only stacks a parent into the render when that parent is registered as a
	// layout (router.Options{Layout:true}), and none here are — so each path
	// resolves to exactly one Shell, no nested-outlet wiring and no duplicated
	// chrome. A Layout/outlet structure was tried but left most rail items
	// un-navigable (child routes rendered outside the Shell / into a missing
	// outlet); reverted. Guarded by screens.TestRailRoutesResolve so a route can't
	// silently fall through to the "*" catch-all again.
	for _, route := range screens.All() {
		route := route // capture per iteration
		r.Register(uistate.RoutePath(route.Path), func(router.Attrs) *router.Element {
			return ui.CreateElement(Shell, ShellProps{
				Title:      uistate.T(route.Title),
				Subtitle:   uistate.T(route.Subtitle),
				ActivePath: route.Path,
				View:       route.View,
			})
		})
	}
	// User-authored custom pages all ride one pattern route; the slug resolves the
	// page from app state at render time, so new pages are reachable without
	// re-registering routes (the router can't be mutated after mount).
	r.Register(uistate.RoutePath("/p/:slug"), func(attrs router.Attrs) *router.Element {
		slug, _ := attrs["slug"].(string)
		title := uistate.T("custompage.fallbackTitle")
		if app := appstate.Default; app != nil {
			if p, ok := pages.BySlug(app.CustomPages(), slug); ok {
				title = p.Name
			}
		}
		return ui.CreateElement(Shell, ShellProps{
			Title:      title,
			ActivePath: "/p/" + slug,
			// Read the slug live (not the captured `slug`) so navigating between two
			// custom pages renders the right one — see liveCustomPageSlug.
			View: func() ui.Node { return screens.CustomPage(liveCustomPageSlug()) },
		})
	})
	// Settings tabs are real, bookmarkable URLs ("/settings/cloud", etc.) rather
	// than a one-shot in-memory deep-link var: one pattern route, same reasoning
	// as "/p/:slug" above — SettingsScreen reads the live tab itself
	// (liveSettingsTab) rather than a captured param, for the identical reason.
	// ActivePath stays the bare "/settings" (not "/settings/"+tab): Shell's
	// WithKey call (shell.go) uses ActivePath as the View subtree's remount
	// key, and Sidebar/TopBar use it for highlighting/breadcrumb — a constant
	// value there keeps the rail correctly showing "Settings" active on every
	// tab and avoids remounting the WHOLE chrome (not just the content) on
	// every tab switch. ContentKey carries the PER-TAB key instead, so only
	// the settings body itself remounts (confirmed live: without a
	// tab-varying key somewhere, the URL updated on every click but the
	// rendered content silently froze after the first tab switch — the
	// reconciler needs a changed key to know the content actually changed).
	r.Register(uistate.RoutePath("/settings/:tab"), func(attrs router.Attrs) *router.Element {
		tab, _ := attrs["tab"].(string)
		return ui.CreateElement(Shell, ShellProps{
			Title:      uistate.T("nav.settings"),
			Subtitle:   uistate.T("screen.settingsSub"),
			ActivePath: "/settings",
			ContentKey: "/settings/" + tab,
			View:       func() ui.Node { return screens.SettingsScreen() },
		})
	})
	// C290: /privacy is a natural URL (and share-crawler target) for the privacy
	// statement, which lives on the About screen. Register it as an explicit alias
	// to the About view so it renders the About & Privacy page instead of silently
	// falling through the "*" catch-all to the dashboard. Not added to screens.All()
	// so it doesn't create a duplicate nav item; ActivePath points at /about so the
	// rail highlights About.
	r.Register(uistate.RoutePath("/privacy"), func(router.Attrs) *router.Element {
		return ui.CreateElement(Shell, ShellProps{
			Title:      uistate.T("nav.about"),
			Subtitle:   uistate.T("screen.aboutSub"),
			ActivePath: "/about",
			View:       screens.About,
		})
	})
	// Unknown paths fall back to the dashboard, still inside the Shell.
	r.Register("*", func(router.Attrs) *router.Element {
		home := screens.All()[0]
		return ui.CreateElement(Shell, ShellProps{Title: uistate.T(home.Title), Subtitle: uistate.T(home.Subtitle), ActivePath: home.Path, View: home.View})
	})

	r.Mount("#app")

	// Wire the dashboard bento drag-reorder coordinator + FLIP reflow animation
	// (the in-Go replacement for the former web/flip.js helper): registers the
	// document-level scroll-lock + drag-retargeting listeners once, now that the DOM
	// host exists.
	uiw.InitBentoCoordinator()

	// Intercept same-origin route-link clicks so a raw <a href> navigates in-app
	// (client-side) instead of doing a full page reload — a reload drops the in-memory
	// app-lock passcode and forces a re-unlock. Covers every page's links at once.
	wireAnchorInterceptor()

	// Global keyboard shortcuts (Alt+1..9 → primary nav sections).
	wireKeyboardShortcuts()

	// Keep the top-bar offline indicator in sync with browser connectivity.
	wireOnlineStatus()

	// If a passcode lock is set, cover the app with the unlock gate, and arm the
	// inactivity auto-lock timer.
	maybeLockOnBoot()
	wireAutoLock()

	// Persist the dataset to localStorage so it survives a reload.
	startDatasetAutosave()
	startBackendSync()
	probeAdminAccess() // non-blocking: sets uistate.AdminConsoleAvailable on HTTP 200

	// Auto-post any due recurring transactions (bills, "pay yourself first")
	// caught up to today, so scheduled money posts the moment the app opens
	// instead of only when the user visits Planning and clicks "Post due". Runs
	// after autosave is armed so the advanced NextDue + new transactions persist
	// immediately (idempotent: NextDue advances past today, so a reopen won't
	// double-post). Boot-safe — a nil/empty store just posts nothing.
	postDueRecurringOnBoot()
	runDueScheduledWorkflowsOnBoot()
	fireBillDueTriggerOnBoot()

	// Expose the music checkpoint bridge (window.cashfluxMusicSave) for the JS player.
	registerMusicBridge()

	// Expose the dataset app-KV bridge (window.cashfluxData*) so the widget-builder
	// canvas shim persists node positions + viewport into the dataset (single source
	// of truth), not a separate browser-store entry.
	registerDatasetKVBridge()

	// Surface "while you were away" reminders (stale balances, bills due soon)
	// once on load, deduped via the persisted delivered log (B19). Boot-safe.
	runNotifyCatchUp()

	// Re-evaluate budget/alert conditions after every user transaction mutation
	// (C122): register an observer so overspend notifications appear immediately
	// after a Quick-Add or delete, not just at next reload. runNotifyCatchUp
	// dedupes by key, so re-running never duplicates already-shown notifications.
	if appstate.Default != nil {
		appstate.Default.OnTxnMutated(func() { runNotifyCatchUp() })
		// Wire the household role guard (R29 / C273): resolve the acting identity's
		// role on every mutation so a member operating the app as a Viewer is held
		// read-only. No selected identity — or an unknown/Owner/Admin one — resolves
		// to RoleOwner (fully permissive), so the default single-user experience is
		// unchanged; only an explicit Viewer identity is restricted.
		appstate.Default.SetActiveRoleFunc(func() domain.MemberRole {
			id := uistate.ActiveIdentityID()
			if id == "" {
				return domain.RoleOwner
			}
			for _, m := range appstate.Default.Members() {
				if m.ID == id {
					return memberrole.Resolve(m)
				}
			}
			return domain.RoleOwner // no match → permissive
		})
		// Persist the Free-feature defaults on first run (C254 / R26). This call
		// is placed here — in the same block where appstate.Default is confirmed
		// non-nil and the store is proven ready — so boot ordering is statically
		// guaranteed: we are already past the store-init seam. The failure mode is
		// benign: if this call were ever skipped, LoadSmartSettings already returns
		// the free-on defaults without persisting (the C254 contract is satisfied
		// via the tier-default path), so no insights are lost.
		uistate.InitSmartSettings()
	}

	// Greet once after a version upgrade with a "what's new" pointer (C326).
	whatsNewToastOnBoot()

	// Readiness signal: boot (hydrate + seed + mount + all wiring) is complete and
	// the dataset is live in memory. Tests and any external harness can wait on
	// `document.documentElement[data-app-ready="true"]` (or `window.__cashfluxReady`)
	// instead of sleeping on a guessed timeout — the single deterministic gate that
	// the whole regression suite keys off. Harmless in production.
	if doc := js.Global().Get("document"); doc.Truthy() {
		if de := doc.Get("documentElement"); de.Truthy() {
			de.Call("setAttribute", "data-app-ready", "true")
		}
	}
	js.Global().Set("__cashfluxReady", true)

	utils.WaitForever()
}
