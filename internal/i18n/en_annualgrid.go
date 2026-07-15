// SPDX-License-Identifier: MIT

package i18n

// annualGridKeys holds English strings for the BG9 annual grid (the view-only
// 12-month plan-vs-actual matrix on /budgets). Kept in a separate file (not en.go)
// so this change doesn't touch the concurrent-WIP catalog. Registered at init.
var annualGridKeys = Catalog{
	"budgets.annualGridTitle":     "Annual grid",
	"budgets.annualGridBudgetCol": "Budget",
	"budgets.annualGridTotalCol":  "Total",
	"budgets.annualGridPrevYear":  "Previous year",
	"budgets.annualGridNextYear":  "Next year",
	"budgets.annualGridError":     "Couldn't build the annual grid.",
}

func init() {
	for k, v := range annualGridKeys {
		english[k] = v
	}
}
