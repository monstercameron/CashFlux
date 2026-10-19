// SPDX-License-Identifier: MIT

package i18n

var bq10a1Keys = Catalog{
	// Each key is a complete sentence so a dialog can show one consequence line
	// whose reach follows the choice that will actually be saved.
	"budgets.impactReachThisPeriod":         "This changes this period only; account balances stay the same, and you can undo it.",
	"budgets.impactReachThisPeriodFunded":   "This changes this period only, moving budgeted money from the budgets you selected; account balances stay the same, and you can undo it.",
	"budgets.impactReachThisAndLater":       "This applies this period and every later period; account balances stay the same, and you can undo it.",
	"budgets.impactReachThisAndLaterFunded": "This applies this period and every later period, moving budgeted money from the budgets you selected; account balances stay the same, and you can undo it.",
	"budgets.impactReachThisAndNext":        "This changes this period and the next one; account balances stay the same, and you can undo it.",
	"budgets.impactReachThisAndNextFunded":  "This changes this period and the next one, moving budgeted money from the budgets you selected; account balances stay the same, and you can undo it.",
	"budgets.impactReachFutureOnly":         "This changes future periods only; the period you are viewing and account balances stay the same, and you can undo it.",
	"budgets.impactReachPermanent":          "This permanently changes the budget limits; account balances stay the same, and you can undo it.",
	"budgets.impactReachPermanentFunded":    "This permanently changes the budget limits and moves budgeted money from the budgets you selected; account balances stay the same, and you can undo it.",
	"budgets.impactReachDelete":             "This removes the budget from this period and every later one; account balances do not change, and you can undo it.",
}

func init() {
	for key, value := range bq10a1Keys {
		english[key] = value
	}
}
