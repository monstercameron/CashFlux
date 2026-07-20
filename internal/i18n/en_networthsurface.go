// SPDX-License-Identifier: MIT

package i18n

// netWorthSurfaceKeys holds the /networth strings that OUTLIVED the bento
// surface: the "as of" line, the composition bucket names (which the new
// balancesheet engine buckets by), the debt drill, and the metrics toggle. The
// strings the old trend/horizon/per-account tiles owned went with those tiles;
// everything the from-scratch balance-sheet surface added lives under `nws.*`
// in en_networthredesign.go. Merged via init so this file does not touch en.go.
var netWorthSurfaceKeys = Catalog{
	// Hero
	"nw.asOf": "Your balance sheet as of %s",

	// Window + drills
	"nw.horizon24":    "2 years",
	"nw.viewDebts":    "View debts",
	"nw.metricsShow":  "Build a custom metric",
	"nw.metricsHide":  "Hide metrics",
	"nw.metricsTitle": "Show every balance-sheet figure as a live formula variable",

	// Series captions
	"nw.labelNow": "Now",

	// Composition
	"nw.ownEmpty":       "No asset accounts yet.",
	"nw.debtFree":       "Debt-free — you owe nothing.",
	"nw.bucketCash":     "Cash",
	"nw.bucketInvested": "Invested",
	"nw.bucketProperty": "Property & vehicles",
	"nw.bucketOther":    "Other assets",
	"nw.bucketCredit":   "Credit cards & lines",
	"nw.bucketLoans":    "Loans",
	"nw.bucketMortgage": "Mortgage",

	// Formula metrics
	"nw.formulaHint": "These balance-sheet figures are live networth_* engine variables — drop any of them into a formula or a dashboard widget.",
}

func init() {
	for k, v := range netWorthSurfaceKeys {
		english[k] = v
	}
}
