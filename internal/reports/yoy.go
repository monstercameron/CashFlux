// SPDX-License-Identifier: MIT

package reports

import (
	"github.com/monstercameron/CashFlux/internal/period"
)

// YoYPrior returns the year-over-year equivalent of w: the same window shifted
// back exactly 12 months, so that the caller can compute a YoY delta by
// comparing the current window against the prior-year window.
//
// Both From and To anchors are shifted with t.AddDate(-1, 0, 0), which follows
// Go's standard calendar arithmetic — including the edge case where Feb 29 in a
// leap year normalises to Mar 1 in a non-leap year (documented behaviour of
// time.AddDate). All other fields (Res, WeekStart) are preserved unchanged.
//
// This is distinct from Window.Shift(-1), which moves the window back one unit
// at its current resolution (month-over-month, quarter-over-quarter, etc.). Use
// YoYPrior when the comparison period must always be exactly one calendar year
// prior regardless of the window's resolution.
func YoYPrior(w period.Window) period.Window {
	return period.Window{
		Res:       w.Res,
		From:      w.From.AddDate(-1, 0, 0),
		To:        w.To.AddDate(-1, 0, 0),
		WeekStart: w.WeekStart,
	}
}
