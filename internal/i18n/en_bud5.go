// SPDX-License-Identifier: MIT

package i18n

// bud5Keys contains the copy for the budget add and raise-limit dialogs.
var bud5Keys = Catalog{
	"budgets.chooseCategory":          "Choose a category",
	"budgets.categoryRequired":        "Choose a category to continue.",
	"budgets.categoryAlreadyBudgeted": "Already budgeted for this period",
	"budgets.raiseLimit":              "Raise limit",
	"budgets.raiseAmount":             "Amount to raise the limit by",
	"budgets.raisePeriodOnly":         "This raises the limit for %s only.",
	"budgets.raisePermanently":        "This raises the limit permanently.",
	// The modal title and submit button use one verb. The row/menu label is
	// declared in en_i18nsweep.go, whose later init overrides this catalog.
	"budgets.topupTitle":     "Raise limit",
	"budgets.addFunds":       "Raise limit",
	"budgets.topupCoverHint": "Checked budgets have their limits lowered by the same amount. Without a source, no other budget changes.",
	"budgets.topupDuration":  "How long should the higher limit last?",
}

func init() {
	for key, value := range bud5Keys {
		english[key] = value
	}
}
