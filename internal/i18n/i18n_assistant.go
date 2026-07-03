// SPDX-License-Identifier: MIT

package i18n

// assistantKeys holds the English strings for the /assistant hub —
// the tabbed AI landing page that merges /insights and /smart into a single
// surface. Kept in a separate file so it can be reviewed and updated without
// touching en.go (the high-churn shared catalog) or en_smart.go.
var assistantKeys = Catalog{
	// Navigation + route spec (Label / Title / Subtitle keys for screens.Route).
	"nav.assistant":       "Assistant",
	"screen.assistantSub": "Chat, spending insights, and smart features — all in one place",

	// Segmented tab labels.
	"assistant.tabGroupLabel": "Assistant section",
	"assistant.tabAsk":        "Ask",
	"assistant.tabInsights":   "Insights",
	"assistant.tabSmart":      "Smart",

	// The Insights briefing surface — hero tile.
	"assistant.heroTitle":       "This month",
	"assistant.heroAsOf":        "Spending through %s",
	"assistant.paceAhead":       "%s ahead of last month's pace",
	"assistant.paceBehind":      "%s behind last month's pace",
	"assistant.paceTitle":       "Compared with what you had spent by this same day last month",
	"assistant.briefQuiet":      "No spending recorded yet this month.",
	"assistant.briefSpent":      "You've spent %s so far this month.",
	"assistant.briefPush":       "%s is doing most of the pushing.",
	"assistant.chipLastMonth":   "Last month in full",
	"assistant.chipTopMerchant": "Top merchant · %s",
	"assistant.chipFlagged":     "Flagged activity",

	// Toolbar.
	"assistant.metricsShow":      "Custom values",
	"assistant.metricsHide":      "Hide custom values",
	"assistant.metricsTitle":     "Build your own figure from the briefing's variables",
	"assistant.viewReports":      "See full reports",
	"assistant.viewTransactions": "View transactions",

	// Tile empty states — each says something useful instead of vanishing.
	"assistant.seeAllInsights":  "See all in Insights →",
	"assistant.flaggedClear":    "No anomalies in your recent activity. (Bill and goal findings live on the Smart tab.)",
	"assistant.highlightsEmpty": "No big category shifts this month.",
	"assistant.merchantsEmpty":  "No spending in the last 90 days yet.",
	"assistant.pinnedEmpty":     "Nothing pinned yet — pin an answer from Ask and it will stay here.",
	"assistant.trendEmpty":      "Once a couple of months of spending are recorded, the trend appears here.",

	// Trend takeaway.
	"assistant.trendAbove": "%s came in %s above your six-month average.",
	"assistant.trendBelow": "%s came in %s below your six-month average.",
	"assistant.trendEven":  "%s landed right on your six-month average.",

	// Formula tile + whole-surface empty state.
	"assistant.formulaHint": "These briefing figures are also formula variables (assistant_…) — build your own metric from them.",
	"assistant.emptyData":   "Add an account and a few transactions first — the briefing works best with real data.",
}

func init() {
	for k, v := range assistantKeys {
		english[k] = v
	}
}
