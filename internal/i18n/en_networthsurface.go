// SPDX-License-Identifier: MIT

package i18n

// netWorthSurfaceKeys holds the English strings for the redesigned /networth
// bento surface (hero, trend horizon, composition pair, per-account rows).
// Merged via init so this file does not touch en.go; mirrors the
// en_reportsurface.go pattern.
var netWorthSurfaceKeys = Catalog{
	// Hero
	"nw.asOf":         "Your balance sheet as of %s",
	"nw.deltaMonth":   "%s this month",
	"nw.deltaTitle":   "How net worth has moved since the month started",
	"nw.figLiquid":    "Liquid share",
	"nw.figDebtRatio": "Debt-to-asset",

	// Toolbar
	"nw.horizon6":     "6 months",
	"nw.horizon12":    "12 months",
	"nw.horizon24":    "2 years",
	"nw.viewDebts":    "View debts",
	"nw.metricsShow":  "Net-worth metrics",
	"nw.metricsHide":  "Hide metrics",
	"nw.metricsTitle": "Show every balance-sheet figure as a live formula variable",

	// Trend
	"nw.trendTitle":        "Trend",
	"nw.labelNow":          "Now",
	"nw.trendTakeawayUp":   "Up %s over the last %d months — now %s.",
	"nw.trendTakeawayDown": "Down %s over the last %d months — now %s.",
	"nw.trendTakeawayFlat": "Holding steady at %s over the last %d months.",

	// Composition
	"nw.ownTitle":       "What you own",
	"nw.ownEmpty":       "No asset accounts yet.",
	"nw.oweTitle":       "What you owe",
	"nw.debtFree":       "Debt-free — you owe nothing.",
	"nw.bucketCash":     "Cash",
	"nw.bucketInvested": "Invested",
	"nw.bucketProperty": "Property & vehicles",
	"nw.bucketOther":    "Other assets",
	"nw.bucketCredit":   "Credit cards & lines",
	"nw.bucketLoans":    "Loans",
	"nw.bucketMortgage": "Mortgage",

	// Accounts
	"nw.accountsTitle": "By account",
	"nw.accountsHint":  "Every account's contribution — bar length is its share of your largest balance.",
	"nw.accountsMore":  "+ %d more accounts",

	// Metrics tile
	"nw.formulaHint": "These balance-sheet figures are live networth_* engine variables — drop any of them into a formula or a dashboard widget.",
}

func init() {
	for k, v := range netWorthSurfaceKeys {
		english[k] = v
	}
}
