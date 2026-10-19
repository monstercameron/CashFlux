// SPDX-License-Identifier: MIT

package i18n

// bv7iKeys refines existing Budgets copy and adds the complete accessible name
// for the inline limit control.
var bv7iKeys = Catalog{
	"budgets.headlineAttention":          "Budgets needing attention: %d",
	"budgets.trackAllCategoryCount":      "Review untracked categories: %d",
	"budgets.fillAvg3":                   "3-period average spend",
	"budgets.fillAvg6":                   "6-period average spend",
	"budgets.rowPrimary":                 "%s, %s",
	"budgets.rowSecondary":               "%s, %d%% used",
	"budgets.limitEditAria":              "%s, change %s limit",
	"budgets.actionEditFormulasPlain":    "Calculate limit with formula",
	"budgets.pauseBudgetForMonth":        "Pause for one month",
	"budgets.recurring.billFoldOne":      "%[2]s a month, %[1]d bill",
	"budgets.recurring.billFoldMany":     "%[2]s a month, %[1]d bills",
	"budgets.planYearStableHint":         "%d plan, %s",
	"budgets.smartNavLabel":              "Smart features",
	"budgets.directionSpend":             "Spending: stay under the amount",
	"budgets.raisePermanently":           "This raises the limit permanently.",
	"budgets.impactReachPermanent":       "This permanently changes this budget's limit; account balances stay the same, and you can undo it.",
	"budgets.impactReachPermanentFunded": "This permanently changes this budget's limit and moves budgeted money from the budgets you selected; account balances stay the same, and you can undo it.",
	"budgets.adjustAllFieldHint":         "A positive number raises every limit, a negative one lowers it: 5 for 5%% more, −10 for 10%% less. Between %s%% and %s%%.",
	"budgets.adjustAllNotANumber":        "Enter a number, like 5 or −10.",
	"bud2.sweep":                         "Sweep leftovers",
	"smart.u1.title":                     "Unusually large charge from %s",
}

func init() {
	for key, value := range bv7iKeys {
		english[key] = value
	}
}
