//go:build js && wasm

// Package screens holds the CashFlux screen registry and the (currently stub)
// view components for each screen. As features land, each stub is replaced by a
// real implementation — ideally split into its own file in this package.
package screens

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Route describes one screen: its URL, nav label, page heading, and view.
type Route struct {
	Path     string
	Label    string
	Title    string
	Subtitle string
	Phase    int
	View     func() ui.Node
}

// All returns the ordered screen registry that drives both routing and the nav.
func All() []Route {
	return []Route{
		{Path: "/", Label: "Dashboard", Title: "Dashboard", Subtitle: "Your money at a glance", Phase: 1, View: Dashboard},
		{Path: "/accounts", Label: "Accounts", Title: "Accounts", Subtitle: "Everything you own and owe", Phase: 1, View: Accounts},
		{Path: "/transactions", Label: "Transactions", Title: "Transactions", Subtitle: "Record income, expenses, and transfers", Phase: 1, View: Transactions},
		{Path: "/budgets", Label: "Budgets", Title: "Budgets", Subtitle: "Individual and group spending limits", Phase: 1, View: Budgets},
		{Path: "/goals", Label: "Goals", Title: "Goals", Subtitle: "Save toward what matters", Phase: 1, View: Goals},
		{Path: "/todo", Label: "To-do", Title: "To-do", Subtitle: "Budgeting tasks and reminders", Phase: 1, View: Todo},
		{Path: "/planning", Label: "Planning", Title: "Planning", Subtitle: "Scenarios and projections", Phase: 2, View: Planning},
		{Path: "/allocate", Label: "Allocate", Title: "Allocate", Subtitle: "Where to put your money next", Phase: 2, View: Allocate},
		{Path: "/insights", Label: "Insights", Title: "Insights", Subtitle: "AI analysis and advice", Phase: 2, View: Insights},
		{Path: "/documents", Label: "Documents", Title: "Documents", Subtitle: "Import statements and receipts with AI", Phase: 2, View: Documents},
		{Path: "/customize", Label: "Customize", Title: "Customize", Subtitle: "Custom fields and formulas", Phase: 2, View: Customize},
		{Path: "/members", Label: "Members", Title: "Members", Subtitle: "Your household", Phase: 1, View: Members},
		{Path: "/categories", Label: "Categories", Title: "Categories", Subtitle: "Income and expense categories", Phase: 1, View: Categories},
		{Path: "/settings", Label: "Settings", Title: "Settings", Subtitle: "Members, currency, AI, and preferences", Phase: 1, View: Settings},
	}
}

// stub renders a consistent placeholder for a not-yet-built screen.
func stub(phase int, description string, points ...string) ui.Node {
	return Section(Class("card"),
		Div(Class("badge badge-soon"), Textf("Planned · Phase %d", phase)),
		P(Class("muted"), description),
		If(len(points) > 0,
			Ul(Class("muted"), MapKeyed(points, func(s string) any { return s }, func(s string) ui.Node { return Li(s) })),
		),
	)
}

func stat(label, value, accent string) ui.Node {
	return Div(Class("stat"),
		Div(Class("stat-label"), label),
		Div(Class("stat-value "+accent), value),
	)
}

