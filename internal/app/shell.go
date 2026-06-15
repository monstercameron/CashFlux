//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// ShellProps configures the chrome around a screen.
type ShellProps struct {
	Title    string
	Subtitle string
	View     func() ui.Node
}

// Shell renders the persistent nav bar and the active screen's content.
func Shell(props ShellProps) ui.Node {
	return Main(Class("app"),
		ui.CreateElement(NavBar),
		Div(Class("page"),
			Header(Class("page-head"),
				H1(Class("page-title"), props.Title),
				P(Class("page-sub"), props.Subtitle),
			),
			ui.CreateElement(props.View),
		),
	)
}

// NavBar is the top navigation, built from the screen registry with the active
// route highlighted.
func NavBar() ui.Node {
	current := router.InspectCurrentRoute().Path
	return Header(Class("topbar"),
		Div(Class("brand"),
			Span(Class("brand-mark"), "$"),
			Span(Class("brand-name"), "CashFlux"),
		),
		Nav(Class("nav"),
			MapKeyed(screens.All(),
				func(r screens.Route) any { return r.Path },
				func(r screens.Route) ui.Node {
					return ui.CreateElement(navLink, navLinkProps{Label: r.Label, Path: r.Path, Active: current == r.Path})
				},
			),
		),
	)
}

type navLinkProps struct {
	Label  string
	Path   string
	Active bool
}

// navLink is its own component so its click handler hook stays stable regardless
// of how the nav list changes (see the On*-hooks-in-loops rule).
func navLink(props navLinkProps) ui.Node {
	nav := router.UseNavigate()
	cls := "nav-link"
	if props.Active {
		cls = "nav-link active"
	}
	return Button(
		Class(cls),
		Type("button"),
		OnClick(func() { nav.Navigate(props.Path) }),
		props.Label,
	)
}
