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

	r := router.NewHistoryRouter(router.RouterOptions{DefaultRoute: "/"})
	for _, route := range screens.All() {
		route := route // capture per iteration
		r.Register(route.Path, func(router.Attrs) *router.Element {
			return ui.CreateElement(Shell, ShellProps{
				Title:    route.Title,
				Subtitle: route.Subtitle,
				View:     route.View,
			})
		})
	}
	// User-authored custom pages all ride one pattern route; the slug resolves the
	// page from app state at render time, so new pages are reachable without
	// re-registering routes (the router can't be mutated after mount).
	r.Register("/p/:slug", func(attrs router.Attrs) *router.Element {
		slug, _ := attrs["slug"].(string)
		title := "Page"
		if app := appstate.Default; app != nil {
			if p, ok := pages.BySlug(app.CustomPages(), slug); ok {
				title = p.Name
			}
		}
		return ui.CreateElement(Shell, ShellProps{
			Title: title,
			View:  func() ui.Node { return screens.CustomPage(slug) },
		})
	})

	// Unknown paths fall back to the dashboard.
	r.Register("*", func(router.Attrs) *router.Element {
		home := screens.All()[0]
		return ui.CreateElement(Shell, ShellProps{Title: home.Title, Subtitle: home.Subtitle, View: home.View})
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

	utils.WaitForever()
}
