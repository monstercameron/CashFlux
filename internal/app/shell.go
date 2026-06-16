//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/ui"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ShellProps configures the chrome around a screen.
type ShellProps struct {
	Title    string
	Subtitle string
	View     func() uic.Node
}

// Shell renders the candidate-C application chrome: a fixed left rail and an
// independently scrolling main pane with a sticky top bar, wrapping the active
// screen's content. (Ported from design/candidate-c.html.)
func Shell(props ShellProps) uic.Node {
	return Div(Class("flex h-screen overflow-hidden bg-base text-fg font-sans"),
		uic.CreateElement(Sidebar),
		Main(Class("cf-scroll flex-1 min-w-0 overflow-y-auto"),
			uic.CreateElement(TopBar, topBarProps{Title: props.Title}),
			Div(Class("p-[10px]"), uic.CreateElement(props.View)),
		),
	)
}

// railItem is one primary navigation entry: a label, route, and icon name.
type railItem struct {
	Label string
	Path  string
	Icon  string
}

// primaryNav is the candidate-C rail's main navigation group.
func primaryNav() []railItem {
	return []railItem{
		{"Dashboard", "/", "dashboard"},
		{"Accounts", "/accounts", "accounts"},
		{"Transactions", "/transactions", "transactions"},
		{"Budgets", "/budgets", "budgets"},
		{"Goals", "/goals", "goals"},
		{"To-do", "/todo", "todo"},
	}
}

// Sidebar renders the left rail: brand header and the primary navigation.
func Sidebar() uic.Node {
	current := router.InspectCurrentRoute().Path
	return Aside(Class("rail w-60 shrink-0 border-r border-line flex flex-col"),
		Div(Class("railhead h-14 flex items-center gap-2.5 px-5 border-b border-line"),
			Span(Class("grid place-items-center w-7 h-7 rounded bg-fg text-base font-display font-semibold text-[13px] shrink-0"), "C"),
			Span(Class("brand-name font-display text-lg font-semibold tracking-tight"), "CashFlux"),
		),
		Nav(Class("flex-1 overflow-y-auto p-3 flex flex-col gap-0.5 text-dim text-[13.5px]"),
			MapKeyed(primaryNav(),
				func(it railItem) any { return it.Path },
				func(it railItem) uic.Node {
					return uic.CreateElement(navItem, navItemProps{
						Label:  it.Label,
						Path:   it.Path,
						Icon:   it.Icon,
						Active: current == it.Path,
					})
				},
			),
		),
	)
}

type navItemProps struct {
	Label  string
	Path   string
	Icon   string
	Active bool
}

// navItem is its own component so its click-handler hook stays stable regardless
// of how the nav list changes (the On*-hooks-in-loops rule).
func navItem(props navItemProps) uic.Node {
	nav := router.UseNavigate()
	cls := "nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer"
	if props.Active {
		cls = "nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer bg-[#1c1c1e] text-fg font-medium"
	}
	return A(
		Class(cls),
		OnClick(func() { nav.Navigate(props.Path) }),
		ui.Icon(props.Icon, Class("w-4 h-4 shrink-0")),
		Span(props.Label),
	)
}

type topBarProps struct {
	Title string
}

// TopBar is the sticky page header inside the scrolling main pane: a (currently
// static) menu toggle, the page title, and an Add action.
func TopBar(props topBarProps) uic.Node {
	return Div(Class("h-14 border-b border-line flex items-center px-6 gap-3 sticky top-0 bg-base z-20"),
		Button(Class("menu-btn w-7 h-7 -ml-1"), Attr("title", "Collapse menu"),
			ui.Icon("menu", Class("w-5 h-5")),
		),
		Div(Class("font-display text-lg font-semibold"), props.Title),
		Div(Class("ml-auto flex items-center gap-2.5 text-dim text-[13px]"),
			Button(Class("px-3 py-1.5 border border-line text-fg hover:bg-hover"), Style(map[string]string{"border-radius": "4px"}),
				"+ Add",
			),
		),
	)
}
