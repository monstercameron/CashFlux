// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app wires routing and the application shell together and mounts the
// CashFlux SPA into the host page. Screen content lives in internal/screens.
package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
	"github.com/monstercameron/GoWebComponents/utils"
)

// Run initializes app state, builds the router, registers every screen wrapped
// in the shell, mounts the app, and blocks the wasm runtime so the page stays
// interactive.
func Run() {
	utils.DisableAllDebug()

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
	hydrateAIKey()

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
			View:       func() ui.Node { return screens.CustomPage(slug) },
		})
	})
	// Unknown paths fall back to the dashboard, still inside the Shell.
	r.Register("*", func(router.Attrs) *router.Element {
		home := screens.All()[0]
		return ui.CreateElement(Shell, ShellProps{Title: uistate.T(home.Title), Subtitle: uistate.T(home.Subtitle), ActivePath: home.Path, View: home.View})
	})

	r.Mount("#app")

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

	// Surface "while you were away" reminders (stale balances, bills due soon)
	// once on load, deduped via the persisted delivered log (B19). Boot-safe.
	runNotifyCatchUp()

	utils.WaitForever()
}
