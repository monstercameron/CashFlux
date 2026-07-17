// SPDX-License-Identifier: MIT

package i18n

// fundShortKeys holds the sinking-fund shortfall alert on the Budgets summary: when
// the month's sinking-fund set-aside is bigger than the money still unallocated, the
// quiet footnote escalates to a planning warning that names the gap and links to the
// goals page where the funds live. Merged via init so this file does not touch en.go.
var fundShortKeys = Catalog{
	"budgets.fundShortTitle":       "Sinking funds are short by %s",
	"budgets.fundShortBody":        "They need %s set aside this month, but only %s is still unallocated.",
	"budgets.fundShortBodyNone":    "They need %s set aside this month, but nothing is still unallocated.",
	"budgets.fundShortReview":      "Review sinking funds",
	"budgets.fundShortReviewTitle": "Open Goals to adjust your sinking funds",
}

func init() {
	for k, v := range fundShortKeys {
		english[k] = v
	}
}
