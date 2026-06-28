// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens holds the CashFlux screen registry and the (currently stub)
// view components for each screen. As features land, each stub is replaced by a
// real implementation — ideally split into its own file in this package.
package screens

import (
	"github.com/monstercameron/GoWebComponents/css"
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
	SubGroup string // sub-section within a group (Tools only); "" for Primary/System
	// AdminOnly, when true, hides this route from the rail for non-admin users
	// (gated by uistate.AdminConsoleAvailable). The route is still registered so
	// a direct URL load works; the shell simply omits it from nav.
	AdminOnly bool
	View      func() ui.Node
}

// Rail navigation groups a screen can belong to. The shell renders one rail
// section per group, in registry order.
const (
	GroupPrimary = "primary" // the main everyday screens
	GroupTools   = "tools"   // Phase-2 power tools
	GroupSystem  = "system"  // household configuration screens
)

// Tools sub-groups (C67): the Tools group is long, so its routes nest into four
// short accordion sub-sections. Membership stays registry-driven (B7) — the shell
// renders by SubGroup; it owns no hardcoded path lists.
const (
	SubGroupPlan       = "plan"       // Plan & forecast: Debt, Investments, Allocate, Planning, Recurring
	SubGroupUnderstand = "understand" // Understand: Reports, NetWorth, Health, Assistant
	SubGroupBills      = "bills"      // retained for const stability; no rail routes currently use it
	SubGroupData       = "data"       // Data & people: Household, Categories, Rules, Artifacts, Activity
	SubGroupBuild      = "build"      // Build: Customize, Fields, Studio, Workflows
)

// ToolsSubGroups is the display order of the Tools sub-sections.
var ToolsSubGroups = []string{SubGroupPlan, SubGroupUnderstand, SubGroupBuild, SubGroupData}

// All returns the ordered screen registry that drives both routing and the nav.
// Label/Title hold i18n keys (resolved by the shell + nav via uistate.T);
// Subtitle holds a screen.*Sub key. The registry carries no display English.
//
// Rail placement is controlled by Group + SubGroup. Routes with no Group are
// routable and deep-linkable but are intentionally omitted from the left rail.
func All() []Route {
	return []Route{
		// PRIMARY — everyday screens, always visible in the top rail section.
		{Path: "/", Label: "nav.dashboard", Title: "nav.dashboard", Subtitle: "screen.dashboardSub", Phase: 1, Group: GroupPrimary, View: Dashboard},
		{Path: "/transactions", Label: "nav.transactions", Title: "nav.transactions", Subtitle: "screen.transactionsSub", Phase: 1, Group: GroupPrimary, View: Transactions},
		{Path: "/accounts", Label: "nav.accounts", Title: "nav.accounts", Subtitle: "screen.accountsSub", Phase: 1, Group: GroupPrimary, View: Accounts},
		{Path: "/budgets", Label: "nav.budgets", Title: "nav.budgets", Subtitle: "screen.budgetsSub", Phase: 1, Group: GroupPrimary, View: Budgets},
		{Path: "/goals", Label: "nav.goals", Title: "nav.goals", Subtitle: "screen.goalsSub", Phase: 1, Group: GroupPrimary, View: Goals},
		{Path: "/todo", Label: "nav.todo", Title: "nav.todo", Subtitle: "screen.todoSub", Phase: 1, Group: GroupPrimary, View: Todo},
		{Path: "/notifications", Label: "nav.notifications", Title: "nav.notifications", Subtitle: "screen.notificationsSub", Phase: 1, Group: GroupPrimary, View: NotificationCenter},

		// TOOLS / Plan & forecast — debt management, investing, allocation, forecasting.
		{Path: "/debt", Label: "nav.debt", Title: "nav.debt", Subtitle: "screen.debtSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: DebtPlanner},
		{Path: "/investments", Label: "nav.investments", Title: "nav.investments", Subtitle: "screen.investmentsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: InvestmentsScreen},
		{Path: "/allocate", Label: "nav.allocate", Title: "nav.allocate", Subtitle: "screen.allocateSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Allocate},
		{Path: "/planning", Label: "nav.planning", Title: "nav.planning", Subtitle: "screen.planningSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Planning},
		{Path: "/recurring", Label: "nav.recurring", Title: "nav.recurring", Subtitle: "screen.recurringSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Recurring},

		// TOOLS / Understand — reporting, net worth, health, and AI assistant.
		{Path: "/reports", Label: "nav.reports", Title: "nav.reports", Subtitle: "screen.reportsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupUnderstand, View: Reports},
		{Path: "/networth", Label: "nav.netWorth", Title: "nav.netWorth", Subtitle: "screen.netWorthSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupUnderstand, View: NetWorth},
		{Path: "/health", Label: "nav.health", Title: "nav.health", Subtitle: "screen.healthSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupUnderstand, View: HealthScreen},
		{Path: "/assistant", Label: "nav.assistant", Title: "nav.assistant", Subtitle: "screen.assistantSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupUnderstand, View: Assistant},

		// TOOLS / Build — customization, custom fields, studio, and workflow automation.
		{Path: "/customize", Label: "nav.customize", Title: "nav.customize", Subtitle: "screen.customizeSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupBuild, View: Customize},
		{Path: "/fields", Label: "nav.fields", Title: "nav.fields", Subtitle: "screen.fieldsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupBuild, View: CustomFields},
		{Path: "/studio", Label: "nav.studio", Title: "nav.studio", Subtitle: "screen.studioSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupBuild, View: Studio},
		{Path: "/workflows", Label: "nav.workflows", Title: "nav.workflows", Subtitle: "screen.workflowsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupBuild, View: Workflows},

		// TOOLS / Data & people — household, categories, rules, and data management.
		{Path: "/household", Label: "nav.household", Title: "nav.household", Subtitle: "screen.householdSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupData, View: Household},
		{Path: "/categories", Label: "nav.categories", Title: "nav.categories", Subtitle: "screen.categoriesSub", Phase: 1, Group: GroupTools, SubGroup: SubGroupData, View: Categories},
		{Path: "/rules", Label: "nav.rules", Title: "nav.rules", Subtitle: "screen.rulesSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupData, View: Rules},
		{Path: "/artifacts", Label: "nav.artifacts", Title: "nav.artifacts", Subtitle: "screen.artifactsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupData, View: Artifacts},
		{Path: "/activity", Label: "nav.activity", Title: "nav.activity", Subtitle: "screen.activitySub", Phase: 2, Group: GroupTools, SubGroup: SubGroupData, View: Activity},

		// SYSTEM — household configuration and app meta.
		{Path: "/appearance", Label: "nav.appearance", Title: "nav.appearance", Subtitle: "screen.appearanceSub", Phase: 1, Group: GroupSystem, View: Appearance},
		{Path: "/help", Label: "nav.help", Title: "nav.help", Subtitle: "screen.helpSub", Phase: 1, Group: GroupSystem, View: HelpScreen},
		{Path: "/about", Label: "nav.about", Title: "nav.about", Subtitle: "screen.aboutSub", Phase: 1, Group: GroupSystem, View: About},
		{Path: "/admin", Label: "nav.admin", Title: "nav.admin", Subtitle: "screen.adminSub", Phase: 2, Group: GroupSystem, AdminOnly: true, View: AdminConsole},
		// C21: Guided setup wizard — walks new users through currency, income, first
		// account, and household members. In GroupSystem so it appears in nav and is
		// reachable from empty-state CTAs.
		{Path: "/setup", Label: "nav.setup", Title: "setup.pageTitle", Subtitle: "setup.pageSub", Phase: 1, Group: GroupSystem, View: SetupWizard},

		// OFF-RAIL — routable and deep-linkable but intentionally absent from the nav.
		// No Label so navGroup skips them; Title/Subtitle preserved for page headings;
		// Phase preserved for filtering. These are consolidated sub-routes: the content
		// is reachable from their hub page (/debt, /recurring, /assistant, /household,
		// /studio) but also via direct URL for bookmarks and deep-links.
		{Path: "/credit", Title: "nav.credit", Subtitle: "screen.creditSub", Phase: 2, View: CreditScreen},
		{Path: "/loans", Title: "nav.loans", Subtitle: "screen.loansSub", Phase: 2, View: LoansScreen},
		{Path: "/bills", Title: "nav.bills", Subtitle: "screen.billsSub", Phase: 2, View: Bills},
		{Path: "/subscriptions", Title: "nav.subscriptions", Subtitle: "screen.subscriptionsSub", Phase: 2, View: Subscriptions},
		{Path: "/insights", Title: "nav.insights", Subtitle: "screen.insightsSub", Phase: 2, View: Insights},
		{Path: "/smart", Title: "nav.smart", Subtitle: "screen.smartSub", Phase: 2, View: SmartHub},
		{Path: "/members", Title: "nav.members", Subtitle: "screen.membersSub", Phase: 1, View: Members},
		{Path: "/split", Title: "nav.split", Subtitle: "screen.splitSub", Phase: 2, View: Split},
		{Path: "/widget-builder", Title: "nav.widgetBuilder", Subtitle: "screen.widgetBuilderSub", Phase: 2, View: VisualBuilder},
		{Path: "/widget-manager", Title: "nav.widgetManager", Subtitle: "screen.widgetManagerSub", Phase: 2, View: WidgetManager},
		{Path: "/documents", Title: "nav.documents", Subtitle: "screen.documentsSub", Phase: 2, View: Documents},
		{Path: "/duplicates", Title: "nav.duplicates", Subtitle: "screen.duplicatesSub", Phase: 2, View: DuplicatesScreen},
		// R31-plans: Plans comparison surface — reachable via the upgrade sheet, cloud
		// mention, and direct navigation.
		{Path: "/plans", Title: "plans.pageTitle", Subtitle: "plans.pageSub", Phase: 1, View: Plans},
	}
}

func stat(label, value, accent string) ui.Node {
	return Div(css.Class("stat"),
		Div(css.Class("stat-label"), label),
		Div(ClassStr("stat-value "+accent), value),
	)
}
