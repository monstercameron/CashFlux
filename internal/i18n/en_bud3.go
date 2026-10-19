// SPDX-License-Identifier: MIT

package i18n

var bud3Keys = Catalog{
	"budgets.effectiveCapLabel":    "Effective cap",
	"budgets.baseLimitLabel":       "Base limit",
	"budgets.actionRaiseLimit":     "Raise limit",
	"budgets.actionCoverFromOther": "Cover from another budget",
	"budgets.actionEditTracking":   "Edit tracking",
	"budgets.actionEditFormulas":   "Edit formulas",
	"budgets.paceLegend":           "Today's pace: %d%% of the month",
}

func init() {
	for key, value := range bud3Keys {
		english[key] = value
	}
}
