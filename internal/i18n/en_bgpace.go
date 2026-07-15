// SPDX-License-Identifier: MIT

package i18n

// bgPaceKeys holds English strings for the BG3 pacing indicator (the even-pace
// tick + caption on a budget's meter) and BG5 envelope-debt visibility. Kept in a
// separate file (not en.go) so this change doesn't touch the concurrent-WIP
// catalog. Registered into the catalog at init time.
var bgPaceKeys = Catalog{
	// BG3: caption when spending is right on the even-pace line.
	"budgets.paceOnPace": "On pace",
	// BG3: caption when spending has outrun the even-pace line. %s = how far ahead.
	"budgets.paceHot": "Running %s hot",
	// BG3: caption when spending is behind the even-pace line (a cushion). %s = amount under.
	"budgets.paceCool": "%s under pace so far",

	// BG5: what next period starts down when this envelope is overdrawn. %s = next
	// period's name (e.g. "March"), %s = deficit amount (e.g. "$32").
	"budgets.envelopeDebtStart": "Starts %s down %s",

	// BG5: rollover-cap picker on the budget edit form — how much unused budget may
	// carry forward, as a multiple of the period limit.
	"budgets.rolloverCapLabel": "Cap rollover at",
	"budgets.rolloverCapNone":  "No cap",
	"budgets.rolloverCap1x":    "1× the budget",
	"budgets.rolloverCap2x":    "2× the budget",
	"budgets.rolloverCap3x":    "3× the budget",
}

func init() {
	for k, v := range bgPaceKeys {
		english[k] = v
	}
}
