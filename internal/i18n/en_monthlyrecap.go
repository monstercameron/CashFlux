// SPDX-License-Identifier: MIT

package i18n

// monthlyRecapKeys holds the strings for the Monthly Recap dashboard widget
// (CG-S1). The banner deliberately does NOT repeat the hero's headline figures —
// it leads with the month-over-month spend CHANGE and the "where did it go"
// category story. Merged via init so this file never touches en.go.
var monthlyRecapKeys = Catalog{
	"dashboard.monthlyRecap": "Monthly recap",

	"dashboard.recapVsLastLabel": "Spend vs last month",
	"dashboard.recapVsLast":      "vs last month",
	"dashboard.recapWas":         "was %s",

	"dashboard.recapSpent":               "Spent",
	"dashboard.recapSpendNew":            "new this month",
	"dashboard.recapTopCategory":         "Top category",
	"dashboard.recapBiggestExpenseLabel": "Biggest expense",
	"dashboard.recapBiggestChange":       "Biggest change",
	"dashboard.recapUncategorized":       "Uncategorized",

	"dashboard.recapNoSpendLabel":   "No-spend days",
	"dashboard.recapNoSpendSub":     "so far this month",
	"dashboard.recapNoSpendSubDone": "this month",

	"dashboard.recapEmpty": "No activity to recap yet — add a few transactions and your month in review appears here.",
}

func init() {
	for k, v := range monthlyRecapKeys {
		english[k] = v
	}
}
