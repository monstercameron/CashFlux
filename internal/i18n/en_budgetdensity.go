// SPDX-License-Identifier: MIT

package i18n

// budgetDensityKeys holds the /budgets list-density toggle copy: full cards vs. the
// compact one-line list for households with many budgets.
//
// C596: the toggle now keeps ONE name and carries its state in aria-pressed. It
// used to swap its label between "Compact list" and "Card view", which named the
// destination in one state and the current view in the other — and announced
// "Card view, pressed" while the compact list was what you were looking at. The
// tooltip is what changes, spelling out the result of a click in each state.
// Merged via init so this file does not touch en.go.
var budgetDensityKeys = Catalog{
	"budgets.densityCompact":  "Compact list",
	"budgets.densityTitleOff": "Show your budgets as a compact list",
	"budgets.densityTitleOn":  "Showing a compact list — switch back to full cards",
}

func init() {
	for k, v := range budgetDensityKeys {
		english[k] = v
	}
}
