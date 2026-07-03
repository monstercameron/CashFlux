// SPDX-License-Identifier: MIT

package i18n

import "maps"

// i18nSweepKeys holds the English strings moved out of hardcoded UI literals by
// the 2026-07-03 i18n sweep (C361): the screens/app first tranche. Every value
// is byte-identical to the literal it replaced, so rendered English output (and
// e2e text matchers) are unchanged — the strings simply become translatable.
// Defined in their own file and merged via init; mirrors the en_setup.go
// extension-file pattern. The screenlint ratchet
// (internal/screenlint/i18n_hardcoded_test.go) holds the line against new
// hardcoded copy.
var i18nSweepKeys = Catalog{
	// Shared actions.
	"action.done":  "Done",
	"action.clear": "Clear",

	// Global period-resolution picker (top bar).
	"period.week":    "Week",
	"period.month":   "Month",
	"period.quarter": "Quarter",
	"period.year":    "Year",

	// Shared list pager.
	"pager.previous": "Previous page",
	"pager.next":     "Next page",

	// Dashboard tiles.
	"dashboard.widgetLoadFailed": "This widget couldn't load.",
	"dashboard.noDataYet":        "No data yet.",
	"dashboard.seriesValue":      "Value",
	"dashboard.axisTime":         "Time",
	"dashboard.trendChange":      "Change",
	"dashboard.trendRange":       "Range",
	"dashboard.noAccountsYet":    "No accounts yet.",
	"dashboard.nothingToDo":      "Nothing to do — nice.",
	"dashboard.noGoalsYet":       "No goals yet.",
	"dashboard.noBudgetsYet":     "No budgets yet.",

	// /split.
	"split.selectAll":           "Select all",
	"split.clear":               "Clear",
	"split.pickPayerHint":       "Pick who paid to see who owes whom.",
	"split.thisSplit":           "This split",
	"split.whoOwesWhom":         "Who owes whom",
	"split.saveSplit":           "Save split",
	"split.runningBalance":      "Running balance",
	"split.runningBalanceHint":  "Running balance across every saved split.",
	"split.runningBalanceEmpty": "Add a shared expense to see who owes whom.",
	"split.squareUpHint":        "Simplest way to square up:",
	"split.allSettled":          "All settled up — nobody owes anybody.",

	// /accounts reconcile flow + forms.
	"accounts.statementBalance":    "Statement balance",
	"accounts.statementBalancePh":  "Enter statement balance",
	"accounts.clearedBalanceLabel": "Cleared balance: ",
	"accounts.differenceLabel":     "Difference: ",
	"accounts.reconciledCheck":     "Reconciled ✓",
	"accounts.unclearedHeading":    "Uncleared transactions — mark cleared to reconcile:",
	"accounts.noUncleared":         "No uncleared transactions. Adjust the statement balance to match the cleared balance above.",
	"accounts.hideAdvanced":        "Hide advanced fields",
	"accounts.showAdvanced":        "Show advanced fields",
	// %s = "N account(s)", %s = comma-joined currency codes.
	"accounts.nwExcludes": "Net worth excludes %s — no exchange rate for %s. Add it in Settings to include them.",

	// /networth toolbar.
	"networth.trendHorizon": "Trend horizon",

	// /health + the dashboard health tile.
	"health.firstReading": "First reading — we'll track your trend from here",
	"health.deltaUp":      "▲ %d since last month",
	"health.deltaDown":    "▼ %d since last month",
	"health.deltaFlat":    "No change since last month",
	"health.noDataHint":   "Add income, accounts, or budgets to see your score",
	"health.weakestLabel": "Weakest: ",
	"health.viewSteps":    "View steps →",
	"health.targetLabel":  "Target: %s",

	// /help.
	"help.fullChangelog":    "See the full changelog →",
	"help.worksOfflineBody": "Everything here works offline — CashFlux runs entirely in your browser.",

	// /debt payoff tools.
	"debt.paidOffSince":       "Paid off %s of %s (%d%%) since %s.",
	"debt.resetProgress":      "Reset progress",
	"debt.startTracking":      "Start tracking progress",
	"debt.strategiesMatch":    "Snowball and avalanche match here — add an extra monthly amount above to see them diverge.",
	"debt.burnDownHeading":    "Balance burn-down to zero:",
	"debt.burnDownChartLabel": "Debt balance falling to zero — avalanche vs snowball over %d months",
	"debt.tryExtra":           "Try %s/mo",

	// /documents receipt-draft review.
	"documents.linesReconciled":     "Lines add up to the total — ready to import as one transaction.",
	"documents.linesUnreadable":     "Check the amounts — one couldn't be read as a number.",
	"documents.linesOffBy":          "Lines are off from the total by %s — adjust the lines or the total to import.",
	"documents.storeNamePh":         "Store name (optional)",
	"documents.rowsAlreadyImported": "%s of %s already imported — will be skipped.",
	"documents.startOver":           "Start over",
	"documents.importReceipt":       "Import receipt",

	// Smart proactive digest (row + /smart section). digestRowTitle is distinct
	// from the existing smart.digestTitle ("Smart digest" section heading).
	"smart.digestCadenceAria": "How often to post a digest",
	"smart.digestRowTitle":    "Proactive money digest",
	"smart.digestRowDesc":     "Post a brief summary of your top active insights to the notification feed.",
	"smart.digestSectionDesc": "Get a brief summary of your top money insights posted to your notification feed, on a schedule you choose. Strictly opt-in — nothing posts until you enable it.",

	// /categories.
	"categories.mapTitle":       "Category map",
	"categories.noTransactions": "No transactions",

	// /rules precedence card.
	"rules.orderTitle":      "Rule order",
	"rules.orderHint":       "First match wins, top to bottom.",
	"rules.precedenceLabel": "Rule precedence chain",

	// Budget card actions.
	"budgets.coverBtn": "Cover…",
	"budgets.topupBtn": "Top up…",

	// Shared component library (second sweep pass — components + helpers).
	"ui.table.all":  "All",
	"ui.table.prev": "Prev",
	"ui.table.next": "Next",
	"ui.kbdHint":    "Enter to save · Esc to cancel",

	// Field labels previously passed as bare helper args.
	"members.roleLabel":   "Role",
	"todo.priorityLabel":  "Priority",
	"smart.digestHeading": "Digest",

	// Global settings panel title (uistate.Global target).
	"settings.panelTitle": "Settings",

	// Settings.
	"settings.tileColor":      "Tile color",
	"settings.dateOptISO":     "2026-06-05  (ISO)",
	"settings.dateOptUS":      "06/05/2026  (US)",
	"settings.dateOptEU":      "05/06/2026  (European)",
	"settings.dateOptLong":    "Jun 5, 2026  (Long)",
	"settings.backendToggle":  "Connect to a backend (sync + AI proxy)",
	"settings.backendOffHint": "Backend off — the app stays fully local; no sync or proxy connections are made.",
}

func init() {
	maps.Copy(english, i18nSweepKeys)
}
