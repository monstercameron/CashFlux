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
		// Label/Title hold i18n keys (resolved by the shell + nav via uistate.T);
		// Subtitle holds a screen.*Sub key. The registry carries no display English.
		{Path: "/", Label: "nav.dashboard", Title: "nav.dashboard", Subtitle: "screen.dashboardSub", Phase: 1, Group: GroupPrimary, View: Dashboard},
		{Path: "/accounts", Label: "nav.accounts", Title: "nav.accounts", Subtitle: "screen.accountsSub", Phase: 1, Group: GroupPrimary, View: Accounts},
		{Path: "/transactions", Label: "nav.transactions", Title: "nav.transactions", Subtitle: "screen.transactionsSub", Phase: 1, Group: GroupPrimary, View: Transactions},
		{Path: "/budgets", Label: "nav.budgets", Title: "nav.budgets", Subtitle: "screen.budgetsSub", Phase: 1, Group: GroupPrimary, View: Budgets},
		{Path: "/goals", Label: "nav.goals", Title: "nav.goals", Subtitle: "screen.goalsSub", Phase: 1, Group: GroupPrimary, View: Goals},
		{Path: "/todo", Label: "nav.todo", Title: "nav.todo", Subtitle: "screen.todoSub", Phase: 1, Group: GroupPrimary, View: Todo},
		{Path: "/planning", Label: "nav.planning", Title: "nav.planning", Subtitle: "screen.planningSub", Phase: 2, Group: GroupTools, View: Planning},
		{Path: "/allocate", Label: "nav.allocate", Title: "nav.allocate", Subtitle: "screen.allocateSub", Phase: 2, Group: GroupTools, View: Allocate},
		{Path: "/reports", Label: "nav.reports", Title: "nav.reports", Subtitle: "screen.reportsSub", Phase: 2, Group: GroupTools, View: Reports},
		{Path: "/subscriptions", Label: "nav.subscriptions", Title: "nav.subscriptions", Subtitle: "screen.subscriptionsSub", Phase: 2, Group: GroupTools, View: Subscriptions},
		{Path: "/bills", Label: "nav.bills", Title: "nav.bills", Subtitle: "screen.billsSub", Phase: 2, Group: GroupTools, View: Bills},
		{Path: "/split", Label: "nav.split", Title: "nav.split", Subtitle: "screen.splitSub", Phase: 2, Group: GroupTools, View: Split},
		{Path: "/insights", Label: "nav.insights", Title: "nav.insights", Subtitle: "screen.insightsSub", Phase: 2, Group: GroupTools, View: Insights},
		{Path: "/documents", Label: "nav.documents", Title: "nav.documents", Subtitle: "screen.documentsSub", Phase: 2, Group: GroupTools, View: Documents},
		{Path: "/customize", Label: "nav.customize", Title: "nav.customize", Subtitle: "screen.customizeSub", Phase: 2, Group: GroupTools, View: Customize},
		{Path: "/artifacts", Label: "nav.artifacts", Title: "nav.artifacts", Subtitle: "screen.artifactsSub", Phase: 2, Group: GroupTools, View: Artifacts},
		{Path: "/workflows", Label: "nav.workflows", Title: "nav.workflows", Subtitle: "screen.workflowsSub", Phase: 2, Group: GroupTools, View: Workflows},
		{Path: "/members", Label: "nav.members", Title: "nav.members", Subtitle: "screen.membersSub", Phase: 1, Group: GroupSystem, View: Members},
		{Path: "/categories", Label: "nav.categories", Title: "nav.categories", Subtitle: "screen.categoriesSub", Phase: 1, Group: GroupSystem, View: Categories},
		{Path: "/rules", Label: "nav.rules", Title: "nav.rules", Subtitle: "screen.rulesSub", Phase: 2, Group: GroupSystem, View: Rules},
	}
}

func stat(label, value, accent string) ui.Node {
	return Div(Class("stat"),
		Div(Class("stat-label"), label),
		Div(Class("stat-value "+accent), value),
	)
}
