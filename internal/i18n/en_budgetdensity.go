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
	// The label names the DESTINATION — what a click produces — which is the
	// conventional reading of a button (Cam, 2026-08-31). C596 had frozen it
	// because a moving label beside aria-pressed announced "Card view, pressed"
	// over a compact list; the fix for that is to drop aria-pressed rather than to
	// freeze the label, since "pressed" was the half that could not be made true in
	// both states. The current view now rides in the accessible name instead, and
	// in data-density for anything that needs to read the state.
	"budgets.densityCompact":  "Compact list",
	"budgets.densityCards":    "Full cards",
	"budgets.densityTitleOff": "Show your budgets as a compact list",
	"budgets.densityTitleOn":  "Show your budgets as full cards",
}

func init() {
	for k, v := range budgetDensityKeys {
		english[k] = v
	}
}

// goalGlyphKeys names the compact row's goal-funded marker once it drops to a
// symbol. The pill's sentence stays the marker's accessible name and the
// popover's body; this is only the popover's heading.
var goalGlyphKeys = Catalog{
	"budgets.goalFundedGlyphTitle": "Goal-funded",
}

func init() {
	for k, v := range goalGlyphKeys {
		english[k] = v
	}
}
