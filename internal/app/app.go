//go:build js && wasm

// Package app wires routing and the application shell together and mounts the
// CashFlux SPA into the host page. Screen content lives in internal/screens.
package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
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

	// Seed an in-memory store with sample data on boot. Logs (os.Stderr) surface
	// in the browser console.
	if err := appstate.Init(nil, true); err != nil {
		panic(err)
	}

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
	// Unknown paths fall back to the dashboard.
	r.Register("*", func(router.Attrs) *router.Element {
		home := screens.All()[0]
		return ui.CreateElement(Shell, ShellProps{Title: home.Title, Subtitle: home.Subtitle, View: home.View})
	})

	r.Mount("#app")
	utils.WaitForever()
}
