// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens holds the CashFlux screen registry and the (currently stub)
// view components for each screen. As features land, each stub is replaced by a
// real implementation — ideally split into its own file in this package.
package screens

import (
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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

// Rail sections. EVERY rail screen carries one, whichever Group it belongs to —
// the rail renders sections, and Group now only feeds the command palette and the
// keyboard fallbacks.
//
// The previous cut grouped by how the app was built rather than by what someone
// came to do, and it showed:
//
//   - Analysis was split THREE ways. Reports sat with the daily screens, Net worth
//     and Financial health sat under "Understand", and "Planning — scenarios and
//     projections" sat under "Plan & forecast". One question, three places.
//   - "Build" was a heading over a single row.
//   - "Data & people" held three unrelated things: people (Household), how
//     transactions get classified (Categories, Rules), and system records
//     (Artifacts, Activity).
//   - The primary group was a grab-bag — the daily money loop next to a chat
//     assistant, an inbox and a task list.
//   - "Understand" and "Build" name what the software does, not what the reader
//     wants. A section label is a signpost; it has to be in their words.
//
// The sections below are the questions a household actually asks, in the order
// they ask them: what happened, what did I decide, what does it mean, what now,
// and how is this set up.
const (
	SubGroupDaily    = "daily"    // What happened: Dashboard, Accounts, Transactions, Bills
	SubGroupPlan     = "plan"     // What I decided ahead: Budgets, Goals, Allocate, Debt, Planning, Events
	SubGroupInsights = "insights" // What it means: Reports, Net worth, Health, Investments
	SubGroupNext     = "next"     // What needs me: Fix My Finances, Assistant, To-do, Notifications
	SubGroupSetup    = "setup"    // How this is set up: categories, rules, household, system

	// Retained so older references keep compiling; no rail route uses them.
	SubGroupUnderstand = "understand"
	SubGroupBills      = "bills"
	SubGroupData       = "data"
	SubGroupBuild      = "build"
)

// NavSections is the rail's section order: daily first because it is opened most,
// setup last because it is opened least, and the three in between follow the
// money's own story rather than the codebase's.
var NavSections = []string{SubGroupDaily, SubGroupPlan, SubGroupInsights, SubGroupNext, SubGroupSetup}

// ToolsSubGroups is kept for callers that still walk the old Tools sub-sections.
var ToolsSubGroups = NavSections

// All returns the ordered screen registry that drives both routing and the nav.
// Label/Title hold i18n keys (resolved by the shell + nav via uistate.T);
// Subtitle holds a screen.*Sub key. The registry carries no display English.
//
// Rail placement is controlled by Group + SubGroup. Routes with no Group are
// routable and deep-linkable but are intentionally omitted from the left rail.
func All() []Route {
	return []Route{
		// PRIMARY — everyday screens, always visible in the top rail section.
		{Path: "/", Label: "nav.dashboard", Title: "nav.dashboard", Subtitle: "screen.dashboardSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupDaily, View: Dashboard},
		{Path: "/transactions", Label: "nav.transactions", Title: "nav.transactions", Subtitle: "screen.transactionsSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupDaily, View: Transactions},
		{Path: "/accounts", Label: "nav.accounts", Title: "nav.accounts", Subtitle: "screen.accountsSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupDaily, View: Accounts},
		{Path: "/budgets", Label: "nav.budgets", Title: "nav.budgets", Subtitle: "screen.budgetsSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupPlan, View: Budgets},
		{Path: "/goals", Label: "nav.goals", Title: "nav.goals", Subtitle: "screen.goalsSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupPlan, View: Goals},
		{Path: "/todo", Label: "nav.todo", Title: "nav.todo", Subtitle: "screen.todoSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupNext, View: Todo},
		{Path: "/notifications", Label: "nav.notifications", Title: "nav.notifications", Subtitle: "screen.notificationsSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupNext, View: NotificationCenter},
		{Path: "/assistant", Label: "nav.assistant", Title: "nav.assistant", Subtitle: "screen.assistantSub", Phase: 1, Group: GroupPrimary, SubGroup: SubGroupNext, View: Assistant},
		// Reports promoted from Tools→Understand to the primary rail (slot 9, so it
		// picks up the Alt+9 jump + digit badge like its siblings).
		{Path: "/reports", Label: "nav.reports", Title: "nav.reports", Subtitle: "screen.reportsSub", Phase: 2, Group: GroupPrimary, SubGroup: SubGroupInsights, View: Reports},

		// TOOLS / Plan & forecast — debt management, investing, allocation, forecasting.
		{Path: "/plan", Label: "nav.plan", Title: "nav.plan", Subtitle: "screen.planSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupNext, View: PlanScreen},
		{Path: "/debt", Label: "nav.debt", Title: "nav.debt", Subtitle: "screen.debtSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: DebtPlanner},
		{Path: "/investments", Label: "nav.investments", Title: "nav.investments", Subtitle: "screen.investmentsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupInsights, View: InvestmentsScreen},
		{Path: "/allocate", Label: "nav.allocate", Title: "nav.allocate", Subtitle: "screen.allocateSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Allocate},
		{Path: "/planning", Label: "nav.planning", Title: "nav.planning", Subtitle: "screen.planningSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Planning},
		{Path: "/recurring", Label: "nav.recurring", Title: "nav.recurring", Subtitle: "screen.recurringSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupDaily, View: Recurring},
		{Path: "/events", Label: "nav.events", Title: "nav.events", Subtitle: "screen.eventsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupPlan, View: Events},

		// TOOLS / Understand — reporting, net worth, and health. (The AI assistant
		// moved to PRIMARY — it's an everyday surface, not a report.)
		{Path: "/networth", Label: "nav.netWorth", Title: "nav.netWorth", Subtitle: "screen.netWorthSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupInsights, View: NetWorth},
		{Path: "/health", Label: "nav.health", Title: "nav.health", Subtitle: "screen.healthSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupInsights, View: HealthScreen},

		// TOOLS / Build — the Studio hub is the one rail entry: formulas, custom
		// fields, and workflows live as its tabs (their standalone routes are
		// off-rail below, kept for bookmarks and deep links).
		{Path: "/studio", Label: "nav.studio", Title: "nav.studio", Subtitle: "screen.studioSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupSetup, View: Studio},

		// TOOLS / Data & people — household, categories, rules, and data management.
		{Path: "/household", Label: "nav.household", Title: "nav.household", Subtitle: "screen.householdSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupSetup, View: Household},
		{Path: "/categories", Label: "nav.categories", Title: "nav.categories", Subtitle: "screen.categoriesSub", Phase: 1, Group: GroupTools, SubGroup: SubGroupSetup, View: Categories},
		{Path: "/rules", Label: "nav.rules", Title: "nav.rules", Subtitle: "screen.rulesSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupSetup, View: Rules},
		{Path: "/artifacts", Label: "nav.artifacts", Title: "nav.artifacts", Subtitle: "screen.artifactsSub", Phase: 2, Group: GroupTools, SubGroup: SubGroupSetup, View: Artifacts},
		{Path: "/activity", Label: "nav.activity", Title: "nav.activity", Subtitle: "screen.activitySub", Phase: 2, Group: GroupTools, SubGroup: SubGroupSetup, View: Activity},

		// SYSTEM — household configuration and app meta.
		{Path: "/settings", Label: "nav.settings", Title: "nav.settings", Subtitle: "screen.settingsSub", Phase: 1, Group: GroupSystem, SubGroup: SubGroupSetup, View: SettingsScreen},
		{Path: "/help", Label: "nav.help", Title: "nav.help", Subtitle: "screen.helpSub", Phase: 1, Group: GroupSystem, SubGroup: SubGroupSetup, View: HelpScreen},
		{Path: "/about", Label: "nav.about", Title: "nav.about", Subtitle: "screen.aboutSub", Phase: 1, Group: GroupSystem, SubGroup: SubGroupSetup, View: About},
		{Path: "/admin", Label: "nav.admin", Title: "nav.admin", Subtitle: "screen.adminSub", Phase: 2, Group: GroupSystem, AdminOnly: true, SubGroup: SubGroupSetup, View: AdminConsole},

		// OFF-RAIL — routable and deep-linkable but intentionally absent from the nav.
		// No Label so navGroup skips them; Title/Subtitle preserved for page headings;
		// Phase preserved for filtering. These are consolidated sub-routes: the content
		// is reachable from their hub page (/debt, /recurring, /assistant, /household,
		// /studio) but also via direct URL for bookmarks and deep-links.
		// Appearance lives on the Settings page's Appearance tab; the standalone
		// route stays for bookmarks/deep links. The guided setup wizard (C21) is
		// launched from /help's setup checklist (and empty-state CTAs) instead of
		// holding a rail slot.
		{Path: "/customize", Title: "nav.customize", Subtitle: "screen.customizeSub", Phase: 2, View: Customize},
		{Path: "/fields", Title: "nav.fields", Subtitle: "screen.fieldsSub", Phase: 2, View: CustomFields},
		{Path: "/workflows", Title: "nav.workflows", Subtitle: "screen.workflowsSub", Phase: 2, View: Workflows},
		{Path: "/appearance", Title: "nav.appearance", Subtitle: "screen.appearanceSub", Phase: 1, View: Appearance},
		{Path: "/setup", Title: "setup.pageTitle", Subtitle: "setup.pageSub", Phase: 1, View: SetupWizard},
		{Path: "/credit", Title: "nav.credit", Subtitle: "screen.creditSub", Phase: 2, View: CreditScreen},
		{Path: "/loans", Title: "nav.loans", Subtitle: "screen.loansSub", Phase: 2, View: LoansScreen},
		{Path: "/bills", Title: "nav.bills", Subtitle: "screen.billsSub", Phase: 2, View: Bills},
		{Path: "/subscriptions", Title: "nav.subscriptions", Subtitle: "screen.subscriptionsSub", Phase: 2, View: Subscriptions},
		{Path: "/insights", Title: "nav.insights", Subtitle: "screen.insightsSub", Phase: 2, View: AssistantInsightsRoute},
		{Path: "/smart", Title: "nav.smart", Subtitle: "screen.smartSub", Phase: 2, View: AssistantAutomationsRoute},
		{Path: "/members", Title: "nav.members", Subtitle: "screen.membersSub", Phase: 1, View: Members},
		{Path: "/split", Title: "nav.split", Subtitle: "screen.splitSub", Phase: 2, View: Split},
		{Path: "/widget-builder", Title: "nav.widgetBuilder", Subtitle: "screen.widgetBuilderSub", Phase: 2, View: VisualBuilder},
		{Path: "/widget-manager", Title: "nav.widgetManager", Subtitle: "screen.widgetManagerSub", Phase: 2, View: WidgetManager},
		{Path: "/documents", Title: "nav.documents", Subtitle: "screen.documentsSub", Phase: 2, View: Documents},
		{Path: "/duplicates", Title: "nav.duplicates", Subtitle: "screen.duplicatesSub", Phase: 2, View: DuplicatesScreen},
		// R31-plans: Plans comparison surface — reachable via the upgrade sheet, cloud
		// mention, and direct navigation.
		{Path: "/plans", Title: "plans.pageTitle", Subtitle: "plans.pageSub", Phase: 1, View: Plans},
		// /sync is now just a redirect to /settings/cloud (2026-07-24 unification) —
		// kept off the rail (Settings owns the nav slot) but still routable so old
		// bookmarks and links land somewhere instead of 404ing.
		{Path: "/sync", Title: "sync.pageTitle", Subtitle: "screen.syncSub", Phase: 3, View: SyncScreen},
	}
}

func stat(label, value, accent string) ui.Node {
	return Div(css.Class("stat"),
		Div(css.Class("stat-label"), label),
		Div(ClassStr("stat-value "+accent), value),
	)
}
