// SPDX-License-Identifier: MIT

package i18n

// smartKeys is the SMART-series (per-page intelligence) set of English strings,
// registered at init time and kept separate from en.go (user WIP) like the other
// feature key files (en_home.go / en_enterprise.go / en_a11y.go). These cover the
// /smart hub chrome and the opt-in catalog; the insight titles and reasons
// themselves are produced by the engines in plain English and are not keyed here.
var smartKeys = Catalog{
	// Navigation + screen subtitle.
	"nav.smart":       "Smart",
	"screen.smartSub": "Optional, on-device intelligence for every page",

	// Inline per-page strip.
	"smart.stripTitle": "Smart",
	"smart.viewAll":    "View all",
	// Collapsed "peek" bar (default) — a slim signal that there are alerts for the page.
	"smart.collapse":       "Collapse",
	"smart.peekTools":      "Smart tools",
	"smart.peekAlertsAria": "%d smart alerts for this page — click to show",
	"smart.peekToolsAria":  "Smart tools for this page — click to show",

	// Empty-state helper (AffordanceEmptyState).
	"smart.emptyHint": "Smart has a tip for this section:",

	// Entity overlay (AffordanceOverlay).
	"smart.overlayTitle": "Smart insights",
	"smart.overlayLabel": "Show Smart insights for this item",

	// Dashboard digest widget.
	"smart.digestTitle": "Smart digest",
	"smart.digestEmpty": "Enable Smart features on the Smart page to see cross-page insights here.",

	// Per-feature run controls.
	"smart.schedule": "Schedule (when it runs)",
	"smart.mute":     "Mute",
	"smart.muted":    "Muted",
	"smart.runNow":   "Run now",
	"smart.running":  "Running…",

	// Global controls (manage header).
	"smart.densityLabel":   "Show smart",
	"smart.enableAll":      "Enable all",
	"smart.enableFreeOnly": "Enable free features only",
	"smart.disableAll":     "Disable all",

	// Insights pagination.
	"smart.prevPage": "← Previous",
	"smart.nextPage": "Next →",
	"smart.pageOf":   "Page %d of %d",

	// Inline explainer tooltips (Free, templated).
	"smart.tipNetWorth":         "Everything you own (assets) minus everything you owe (debts) — your bottom-line worth.",
	"smart.tipBudgetSafe":       "How much you can still spend in this category before hitting your budget limit for the period.",
	"smart.tipGoalProgress":     "Total saved across all active goals divided by the combined target — how far you are toward finishing everything.",
	"smart.tipAccountsNet":      "All asset balances minus all liability balances, converted to your base currency — your net position across every account.",
	"smart.tipPlanningForecast": "Where your net worth is projected to land in 12 months if your recent average monthly cash flow continues unchanged.",
	"smart.tipTxnTotal":         "The net of every transaction matching your current filters — income minus spending — in your base currency.",
	"smart.tipBillsDue":         "The total amount owed across all upcoming bills within the current view window, converted to your base currency.",
	"smart.tipSubsMonthly":      "The normalized monthly cost of all active detected subscriptions — what recurring charges add up to each month.",

	// Tab bar.
	"smart.tabInsights": "Insights",
	"smart.tabManage":   "Manage",

	// Insights section.
	"smart.insightsTitle":         "Your insights",
	"smart.onboard":               "Turn on a smart feature below to start seeing optional, on-device insights here. Everything is off until you choose it, and nothing leaves your device unless you enable an AI feature.",
	"smart.allClear":              "All clear — no insights need your attention right now.",
	"smart.dismiss":               "Dismiss",
	"smartcat.title":              "Smart categorization",
	"smartcat.button":             "Categorize",
	"smartcat.modeLabel":          "Mode",
	"smartcat.modeSuggest":        "Suggest",
	"smartcat.modeAuto":           "Auto-fill",
	"smartcat.modeRecat":          "Fix mistakes",
	"smartcat.hintSuggest":        "Scan your uncategorized transactions and propose new categories to create. Nothing is sent until you scan.",
	"smartcat.hintAuto":           "Scan your uncategorized transactions and propose a category for each. You confirm before anything changes.",
	"smartcat.hintRecat":          "Scan your categorized transactions for likely mistakes and propose fixes. You confirm each change.",
	"smartcat.scan":               "Scan",
	"smartcat.rescan":             "Scan again",
	"smartcat.scanning":           "Scanning your transactions…",
	"smartcat.noneSuggest":        "No new categories to suggest — your uncategorized transactions look covered.",
	"smartcat.noneAssign":         "Nothing to suggest right now.",
	"smartcat.createSelected":     "Create selected",
	"smartcat.applySelected":      "Apply selected",
	"smartcat.kindExpense":        "Expense",
	"smartcat.kindIncome":         "Income",
	"smartcat.createdToast":       "Created %s.",
	"smartcat.appliedToast":       "Updated %s.",
	"smartcat.appliedLabel":       "Smart-categorized %d",
	"smart.dismissAll":            "Dismiss all",
	"smart.snoozeDay":             "Snooze for a day",
	"smart.snoozeWeek":            "Snooze for a week",
	"smart.panelActions":          "Smart panel actions",
	"smart.taskAdded":             "Added to your to-dos.",
	"smart.goalCreated":           "Goal created.",
	"smart.sinkingFundCreated":    "Sinking fund created.",
	"smart.recurringCreated":      "Added to your plan.",
	"smart.subscriptionCancelled": "Subscription marked as cancelled.",
	"smart.automateGoalCreated":   "Automatic monthly contribution set up.",

	// Manage / opt-in catalog.
	"smart.manageTitle":   "Manage smart features",
	"smart.manageHint":    "Every feature is optional and off by default. Free features run entirely on your device at no cost; AI features need an inference provider and are billed per use.",
	"smart.tierFree":      "Free",
	"smart.tierAI":        "AI",
	"smart.perUse":        "/use",
	"smart.needsProvider": "needs a provider",

	// AI feature controls.
	"smart.aiTitle":             "Ask & analyze",
	"smart.aiNeedsProvider":     "These AI features need an inference provider. Add an OpenAI key (or connect the hosted backend) in Settings to use them.",
	"smart.aiCostPrefix":        "about",
	"smart.askTitle":            "Ask about your accounts",
	"smart.askPlaceholder":      "e.g. Which account grew the most this year?",
	"smart.ask":                 "Ask",
	"smart.asking":              "Asking…",
	"smart.outlookTitle":        "Summarize my outlook",
	"smart.outlookBtn":          "Summarize",
	"smart.healthTitle":         "Explain my account health",
	"smart.healthBtn":           "Explain",
	"smart.goalTitle":           "Draft a goal from a wish",
	"smart.goalPlaceholder":     "e.g. save for a $6k Japan trip next spring",
	"smart.scenarioTitle":       "Plan a what-if scenario",
	"smart.scenarioPlaceholder": "e.g. what if I get a $500/mo raise and pay $200 extra on my card?",
	"smart.allocTitle":          "Allocate in plain English",
	"smart.allocPlaceholder":    "e.g. put most toward the credit card but keep $1k liquid",
	"smart.overlapTitle":        "Find overlapping subscriptions",
	"smart.overlapBtn":          "Check for overlaps",
	"smart.todoTitle":           "Add a to-do in plain English",
	"smart.todoPlaceholder":     "e.g. move $200 to savings next Friday",

	"smart.cleanupTitle":          "Clean up an account name",
	"smart.cleanupPlaceholder":    "e.g. PLAID-CHK-8842",
	"smart.categorizeTitle":       "Categorize a transaction",
	"smart.categorizePlaceholder": "e.g. SQ *BLUE BOTTLE 8829 SF, $4.50",
	"smart.searchTitle":           "Search in plain English",
	"smart.searchPlaceholder":     "e.g. coffee over $10 last month",
	"smart.merchantTitle":         "Clean up a merchant name",
	"smart.merchantPlaceholder":   "e.g. SQ *BLUE BOTTLE 8829 SF",
	"smart.taxTitle":              "Find tax-relevant transactions",
	"smart.taxBtn":                "Scan for deductions",
	"smart.priorityTitle":         "Which goal to fund first",
	"smart.priorityBtn":           "Recommend an order",
	"smart.benchmarkTitle":        "Is this subscription priced fairly?",
	"smart.benchmarkPlaceholder":  "e.g. Spotify $11/mo",
	"smart.bundleTitle":           "Find bundle opportunities",
	"smart.bundleBtn":             "Check for bundles",
	"smart.importTitle":           "Map CSV columns for import",
	"smart.importPlaceholder":     "paste your CSV header row (e.g. Date,Description,Amount)",
	"smart.receiptTitle":          "Scan a receipt",
	"smart.receiptBtn":            "Snap or upload a receipt",
	"smart.receiptNeedsKey":       "Receipt scanning needs an OpenAI key (vision). Add one in Settings.",
}

func init() {
	for k, v := range smartKeys {
		english[k] = v
	}
}
