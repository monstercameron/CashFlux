// SPDX-License-Identifier: MIT

package i18n

import "maps"

// wSeries5Keys holds the English copy for the 2026-07-19 W-series Budgets rows/styling
// lane: the per-row rollover policy badge + carryover-math popover (C395) and the clearer
// "Smart features" navigation affordance on the budgets list header (C397). Kept in its
// own extension file and merged via init, mirroring the en_i18nsweep.go / en_wseries2.go
// pattern so the screens layer stays at zero hardcoded copy (internal/screenlint ratchet).
var wSeries5Keys = Catalog{
	// --- C395: per-row rollover policy badge + carryover-math popover ---
	// C395 — per-row rollover policy badge (states: off / rolls over / capped N periods).
	"budgetsRollover.badgeOff":    "No rollover",
	"budgetsRollover.badgeOn":     "Rolls over",
	"budgetsRollover.badgeCapped": "Rolls over · cap %d",
	"budgetsRollover.badgeAria":   "Rollover policy: %s. Show this period's carryover math.",

	// C395 — the carryover-math popover (deterministic, no black boxes): each line is
	// built from the same figures the budgeting engine produced for this period.
	"budgetsRollover.popTitle":      "Rollover",
	"budgetsRollover.offText":       "Unused money doesn't carry over. Each period starts fresh at the limit.",
	"budgetsRollover.onIntro":       "Unused money carries into the next period.",
	"budgetsRollover.carryLine":     "Last period ended with %s.",
	"budgetsRollover.noCarryLine":   "Nothing has carried in yet — last period left no surplus or shortfall.",
	"budgetsRollover.capLine":       "Carry-over is capped at %d× the limit, so a cushion can't build up without bound.",
	"budgetsRollover.capThisPeriod": "This period's cap: %s (%s).",

	// --- C397: the budgets list header "Smart" control now reads as navigation ---
	"budgets.smartNavLabel":   "Smart features →",
	"budgets.smartNavTooltip": "Open the Smart page — insights and automations for your budgets",
	"budgets.smartNavAria":    "Smart features — opens the Smart page",
}

func init() {
	maps.Copy(english, wSeries5Keys)
}
