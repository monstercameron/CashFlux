//go:build js && wasm

package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// railCollapsed is the shared atom coordinating the collapsible rail: the top
// bar's menu button toggles it and the sidebar reads it to switch to icon-only
// mode. Keyed globally so both components stay in sync.
const railCollapsedAtom = "rail:collapsed"

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
		uic.CreateElement(SettingsHost),
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

// customPage is an example "My pages" entry. Real, user-created custom pages
// arrive with the custom-pages feature; for now these mirror the mockup.
type customPage struct {
	Label     string
	IconClass string
}

// myPages returns the example custom pages shown in the rail.
func myPages() []customPage {
	return []customPage{
		{"Debt payoff plan", "w-4 h-4 shrink-0 text-down"},
		{"FIRE tracker", "w-4 h-4 shrink-0 text-up"},
		{"Side hustle P&L", "w-4 h-4 shrink-0 text-[#7c83ff]"},
	}
}

// railHeader renders a small uppercase section label inside the rail.
func railHeader(label string) uic.Node {
	return Div(Class("px-3 pt-4 pb-1 text-[10px] uppercase tracking-[0.16em] text-faint"), label)
}

// Sidebar renders the left rail: brand header, primary navigation, the user's
// custom "My pages", the System group, and a household card that opens settings.
func Sidebar() uic.Node {
	current := router.InspectCurrentRoute().Path
	cls := "rail w-60 shrink-0 border-r border-line flex flex-col"
	if state.UseAtom(railCollapsedAtom, false).Get() {
		cls += " collapsed"
	}
	return Aside(Class(cls),
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
			railHeader("My pages"),
			MapKeyed(myPages(),
				func(p customPage) any { return p.Label },
				func(p customPage) uic.Node {
					return uic.CreateElement(navItem, navItemProps{
						Label:     p.Label,
						Icon:      "page",
						IconClass: p.IconClass,
					})
				},
			),
			uic.CreateElement(navItem, navItemProps{Label: "New page", Icon: "plus", Muted: true}),
			railHeader("System"),
			uic.CreateElement(navItem, navItemProps{
				Label:  "Settings",
				Path:   "/settings",
				Icon:   "settings",
				Active: current == "/settings",
			}),
		),
		uic.CreateElement(HouseholdCard),
	)
}

type navItemProps struct {
	Label     string
	Path      string // empty = non-navigating placeholder (e.g. example pages)
	Icon      string
	IconClass string // defaults to "w-4 h-4 shrink-0"
	Active    bool
	Muted     bool // faint styling for low-emphasis actions ("New page")
}

// navItem is its own component so its click-handler hook stays stable regardless
// of how the nav list changes (the On*-hooks-in-loops rule).
func navItem(props navItemProps) uic.Node {
	nav := router.UseNavigate()
	cls := "nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer"
	switch {
	case props.Active:
		cls = "nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer bg-[#1c1c1e] text-fg font-medium"
	case props.Muted:
		cls = "nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer text-faint"
	}
	iconClass := props.IconClass
	if iconClass == "" {
		iconClass = "w-4 h-4 shrink-0"
	}
	path := props.Path
	return A(
		Class(cls),
		OnClick(func() {
			if path != "" {
				nav.Navigate(path)
			}
		}),
		ui.Icon(props.Icon, Class(iconClass)),
		Span(props.Label),
	)
}

// HouseholdCard sits at the bottom of the rail, summarizing the household and
// opening the global settings flip panel on click. It reads live member count
// and base currency from app state.
func HouseholdCard() uic.Node {
	settings := uistate.UseSettings()
	name := "Your household"
	summary := "Settings"
	if app := appstate.Default; app != nil {
		base := app.Settings().BaseCurrency
		if base == "" {
			base = "USD"
		}
		members := len(app.Members())
		noun := "members"
		if members == 1 {
			noun = "member"
		}
		summary = fmt.Sprintf("%d %s · %s base · Settings", members, noun, base)
	}
	return Button(
		Class("hh mt-auto m-3 p-3 rounded-[4px] border border-line flex items-center gap-2.5 text-left hover:bg-hover"),
		OnClick(func() { settings.Set(uistate.Global()) }),
		ui.Icon("settings", Class("w-4 h-4 shrink-0 text-dim")),
		Span(Class("hh-text leading-tight"),
			Span(Class("font-display text-[14px] font-medium block"), name),
			Span(Class("text-xs text-faint block"), summary),
		),
	)
}

type topBarProps struct {
	Title string
}

// TopBar is the sticky page header inside the scrolling main pane: a (currently
// static) menu toggle, the page title, and an Add action.
func TopBar(props topBarProps) uic.Node {
	collapsed := state.UseAtom(railCollapsedAtom, false)
	return Div(Class("h-14 border-b border-line flex items-center px-6 gap-3 sticky top-0 bg-base z-20"),
		Button(Class("menu-btn w-7 h-7 -ml-1"), Attr("title", "Collapse menu"),
			OnClick(func() { collapsed.Update(func(c bool) bool { return !c }) }),
			ui.Icon("menu", Class("w-5 h-5")),
		),
		Div(Class("font-display text-lg font-semibold"), props.Title),
		Div(Class("ml-auto flex items-center gap-2.5 text-dim text-[13px]"),
			uic.CreateElement(ResolutionControl),
			Button(Class("px-3 py-1.5 border border-line text-fg hover:bg-hover"), Style(map[string]string{"border-radius": "4px"}),
				"+ Add",
			),
		),
	)
}

// ResolutionControl is the top bar's time-resolution control: a Week/Month/
// Quarter segmented toggle and From/To stepper pills, all driven by the shared
// dashboard window (internal/uistate + internal/period). It owns no date math —
// every action just stores the next immutable Window.
func ResolutionControl() uic.Node {
	atom := uistate.UsePeriod()
	w := atom.Get()
	return Span(Class("flex items-center gap-2.5"),
		ui.Segmented(ui.SegmentedProps{
			Options: []ui.SegOption{
				{Value: string(period.Week), Label: "Week"},
				{Value: string(period.Month), Label: "Month"},
				{Value: string(period.Quarter), Label: "Quarter"},
			},
			Selected: string(w.Res),
			OnSelect: func(v string) { atom.Set(w.SetResolution(period.Resolution(v))) },
		}),
		ui.StepperPill(ui.StepperPillProps{
			Label:  w.FromLabel(),
			OnPrev: func() { atom.Set(w.StepFrom(-1)) },
			OnNext: func() { atom.Set(w.StepFrom(1)) },
		}),
		Span(Class("text-faint"), "–"),
		ui.StepperPill(ui.StepperPillProps{
			Label:  w.ToLabel(),
			OnPrev: func() { atom.Set(w.StepTo(-1)) },
			OnNext: func() { atom.Set(w.StepTo(1)) },
		}),
	)
}
