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

// Dashboard is the landing screen. Placeholder metrics until the store lands.
func Dashboard() ui.Node {
	return Div(
		Div(Class("stat-grid"),
			stat("Net worth", "—", ""),
			stat("This month in", "—", "pos"),
			stat("This month out", "—", "neg"),
			stat("Accounts", "—", ""),
		),
		stub(1, "The dashboard will summarize net worth, per-member and group rollups, budget health, freshness nudges, your top AI insight, and recent activity."),
	)
}

func stat(label, value, accent string) ui.Node {
	return Div(Class("stat"),
		Div(Class("stat-label"), label),
		Div(Class("stat-value "+accent), value),
	)
}

// Transactions is the global ledger.
func Transactions() ui.Node {
	return stub(1, "The global ledger: add, edit, and delete income, expenses, and transfers, with filters by member, account, category, date, and text.")
}

// Budgets covers individual and group limits.
func Budgets() ui.Node {
	return stub(1, "Set individual (per member) and group budgets, and track spent vs. remaining with clear progress and gentle alerts.")
}

// Goals tracks savings goals.
func Goals() ui.Node {
	return stub(1, "Create individual and group savings goals with progress and a projected completion date.")
}

// Todo is the budgeting task list.
func Todo() ui.Node {
	return stub(1, "A budgeting-related to-do list — open/done, due dates, priority — with items linked to accounts, budgets, or goals.")
}

// Planning is the scenario/projection tool.
func Planning() ui.Node {
	return stub(2, "Build scenarios from recurring items and assumptions, compare against actuals, and push a chosen scenario into the forecast.")
}

// Allocate is the capital-allocation engine.
func Allocate() ui.Node {
	return stub(2, "Enter an amount and pick a profile to get ranked suggestions for where to put your money — scored on stability, returns, ease of withdrawal, and debt reduction, with a clear breakdown.")
}

// Insights is AI analysis.
func Insights() ui.Node {
	return stub(2, "AI-generated analysis and advice (OpenAI, using your own key): explain your month, spot trends, and answer plain-language questions about your money.")
}

// Documents is AI import.
func Documents() ui.Node {
	return stub(2, "Upload statements and receipts; AI extracts transactions for you to review and import, powering effortless monthly-spend tracking.")
}

// Customize covers custom fields and formulas.
func Customize() ui.Node {
	return stub(2, "Extend the app without code: define custom fields on any entity and build your own calculations with the formula builder.")
}

// Settings holds configuration.
func Settings() ui.Node {
	return stub(1, "Members, base currency and exchange-rate table, categories, freshness windows, your OpenAI key and model, data import/export, and more.")
}
