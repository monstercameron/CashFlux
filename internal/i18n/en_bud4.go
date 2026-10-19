// SPDX-License-Identifier: MIT

package i18n

// budgetsFutureKeys is the BUD4 copy for future-month planning. The closed
// disclosure gives a quick bill count and total; opening it distinguishes the
// plan's expected income basis from money already received.
var budgetsFutureKeys = Catalog{
	"budgets.future.foldHintOne":    "%[1]d bill expected · %[2]s",
	"budgets.future.foldHintMany":   "%[1]d bills expected · %[2]s",
	"budgets.future.showAria":       "Show the future-month projection",
	"budgets.future.hideAria":       "Hide the future-month projection",
	"budgets.future.planning":       "Planned %s · expected income %s",
	"budgets.future.received":       "Received so far: %s",
	"budgets.future.expectedIncome": "Expected income",
}

func init() {
	for key, value := range budgetsFutureKeys {
		english[key] = value
	}
}
