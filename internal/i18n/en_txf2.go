// SPDX-License-Identifier: MIT

package i18n

// txf2Keys clarifies transaction filter ranges and the scope count line.
var txf2Keys = Catalog{
	"transactions.chipAmountBound":          "Amount %s %s",
	"transactions.filterRangeAmountInvalid": "Minimum is larger than maximum.",
	"transactions.filterRangeDateInvalid":   "Start date is after end date.",
	"transactions.scopeExcluded":            "includes %d left out of budgets and reports",
}

func init() {
	for key, value := range txf2Keys {
		english[key] = value
	}
}
