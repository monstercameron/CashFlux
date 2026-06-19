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
	hydrateDataset()
	hydrateAIKey()

	// Apply saved appearance preferences (theme/accent/density) before mounting,
	// so the first paint matches the user's choice instead of flashing defaults.
	uistate.ApplyPrefs(uistate.LoadPrefs())

	// Derive the URL sub-path the app is served under (e.g. "/CashFlux" on a
	// GitHub Pages project site) from the <base href> index.html set, so routes
	// register and navigate with that prefix (B30). At the server root the prefix
	// is "" and every RoutePath call is a no-op.
	uistate.InitRouteBase()

	r := router.NewHistoryRouter(router.RouterOptions{DefaultRoute: uistate.RoutePath("/")})
	home := screens.All()[0]
	r.Register(uistate.RoutePath("/"), func(router.Attrs) *router.Element {
		view := home.View
		if outlet := router.GetOutlet(); outlet != nil {
			view = func() ui.Node { return outlet }
		}
		title, subtitle := shellLabelsForCurrentRoute(home.Title, home.Subtitle)
		return ui.CreateElement(Shell, ShellProps{
			Title:    title,
			Subtitle: subtitle,
			View:     view,
		})
	}, router.Options{Layout: true})
	for _, route := range screens.All() {
		if route.Path == "/" {
			continue
		}
		route := route // capture per iteration
		r.Register(uistate.RoutePath(route.Path), func(router.Attrs) *router.Element {
			return route.View()
		})
	}
	// User-authored custom pages all ride one pattern route; the slug resolves the
	// page from app state at render time, so new pages are reachable without
	// re-registering routes (the router can't be mutated after mount).
	r.Register(uistate.RoutePath("/p/:slug"), func(attrs router.Attrs) *router.Element {
		slug, _ := attrs["slug"].(string)
		return screens.CustomPage(slug)
	})

	// Unknown paths fall back to the dashboard.
	r.Register("*", func(router.Attrs) *router.Element {
		return home.View()
	})

	r.Mount("#app")

	// Reveal the widgets' resize handles only while Shift is held.
	wireResizeReveal()

	// Global keyboard shortcuts (Alt+1..9 → primary nav sections).
	wireKeyboardShortcuts()

	// If a passcode lock is set, cover the app with the unlock gate, and arm the
	// inactivity auto-lock timer.
	maybeLockOnBoot()
	wireAutoLock()

	// Persist the dataset to localStorage so it survives a reload.
	startDatasetAutosave()
	startBackendSync()

	// Surface "while you were away" reminders (stale balances, bills due soon)
	// once on load, deduped via the persisted delivered log (B19). Boot-safe.
	runNotifyCatchUp()

	utils.WaitForever()
}

func shellLabelsForCurrentRoute(fallbackTitle, fallbackSubtitle string) (string, string) {
	current := uistate.LogicalPath(router.InspectCurrentRoute().Path)
	for _, route := range screens.All() {
		if route.Path == current {
			return uistate.T(route.Title), uistate.T(route.Subtitle)
		}
	}
	if slug, ok := customPageSlug(current); ok {
		if app := appstate.Default; app != nil {
			if p, ok := pages.BySlug(app.CustomPages(), slug); ok {
				return p.Name, ""
			}
		}
		return uistate.T("custompage.fallbackTitle"), ""
	}
	return uistate.T(fallbackTitle), uistate.T(fallbackSubtitle)
}

func customPageSlug(path string) (string, bool) {
	const prefix = "/p/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return "", false
	}
	return path[len(prefix):], true
}
