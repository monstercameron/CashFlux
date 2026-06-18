//go:build js && wasm

// Package screens holds the CashFlux screen registry and the (currently stub)
// view components for each screen. As features land, each stub is replaced by a
// real implementation — ideally split into its own file in this package.
package screens

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Route describes one screen: its URL, nav label, page heading, and view. Group
// places the screen in a rail navigation section (see the Group* constants); the
// rail is derived from this field, so a newly registered screen can't be silently
// dropped from navigation (B7).
type Route struct {
	Path     string
	Label    string
	Title    string
	Subtitle string
	Phase    int
	Group    string
	View     func() ui.Node
}

// Rail navigation groups a screen can belong to. The shell renders one rail
// section per group, in registry order.
const (
	GroupPrimary = "primary" // the main everyday screens
	GroupTools   = "tools"   // Phase-2 power tools
	GroupSystem  = "system"  // household configuration screens
)

// All returns the ordered screen registry that drives both routing and the nav.
func All() []Route {
	return []Route{
		{Path: "/", Label: "Dashboard", Title: "Dashboard", Subtitle: "Your money at a glance", Phase: 1, Group: GroupPrimary, View: Dashboard},
		{Path: "/accounts", Label: "Accounts", Title: "Accounts", Subtitle: "Everything you own and owe", Phase: 1, Group: GroupPrimary, View: Accounts},
		{Path: "/transactions", Label: "Transactions", Title: "Transactions", Subtitle: "Record income, expenses, and transfers", Phase: 1, Group: GroupPrimary, View: Transactions},
		{Path: "/budgets", Label: "Budgets", Title: "Budgets", Subtitle: "Individual and group spending limits", Phase: 1, Group: GroupPrimary, View: Budgets},
		{Path: "/goals", Label: "Goals", Title: "Goals", Subtitle: "Save toward what matters", Phase: 1, Group: GroupPrimary, View: Goals},
		{Path: "/todo", Label: "To-do", Title: "To-do", Subtitle: "Budgeting tasks and reminders", Phase: 1, Group: GroupPrimary, View: Todo},
		{Path: "/planning", Label: "Planning", Title: "Planning", Subtitle: "Scenarios and projections", Phase: 2, Group: GroupTools, View: Planning},
		{Path: "/allocate", Label: "Allocate", Title: "Allocate", Subtitle: "Where to put your money next", Phase: 2, Group: GroupTools, View: Allocate},
		{Path: "/insights", Label: "Insights", Title: "Insights", Subtitle: "AI analysis and advice", Phase: 2, Group: GroupTools, View: Insights},
		{Path: "/documents", Label: "Documents", Title: "Documents", Subtitle: "Import statements and receipts with AI", Phase: 2, Group: GroupTools, View: Documents},
		{Path: "/customize", Label: "Customize", Title: "Customize", Subtitle: "Custom fields and formulas", Phase: 2, Group: GroupTools, View: Customize},
		{Path: "/artifacts", Label: "Artifacts", Title: "Artifacts", Subtitle: "Images and datasets for your pages", Phase: 2, Group: GroupTools, View: Artifacts},
		{Path: "/workflows", Label: "Workflows", Title: "Workflows", Subtitle: "Automate actions with triggers and rules", Phase: 2, Group: GroupTools, View: Workflows},
		{Path: "/members", Label: "Members", Title: "Members", Subtitle: "Your household", Phase: 1, Group: GroupSystem, View: Members},
		{Path: "/categories", Label: "Categories", Title: "Categories", Subtitle: "Income and expense categories", Phase: 1, Group: GroupSystem, View: Categories},
		{Path: "/rules", Label: "Rules", Title: "Rules", Subtitle: "Auto-categorize transactions by keyword", Phase: 2, Group: GroupSystem, View: Rules},
	}
}

func stat(label, value, accent string) ui.Node {
	return Div(Class("stat"),
		Div(Class("stat-label"), label),
		Div(Class("stat-value "+accent), value),
	)
}
