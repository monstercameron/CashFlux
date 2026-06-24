// SPDX-License-Identifier: MIT

package payoff

// Progress measures debt-payoff progress against a baseline snapshot of what was
// owed when tracking started.
type Progress struct {
	Baseline  int64 // total owed when tracking started, minor units
	Current   int64 // total owed now, minor units
	PaidOff   int64 // baseline - current, clamped at >= 0
	Remaining int64 // current owed, clamped at >= 0
	Percent   int   // paid-off as a percent of the baseline, 0..100
}

// TrackProgress computes payoff progress from a baseline total and the current
// total owed (both minor units). Paid-off is clamped at >= 0 so a balance that
// grew reads 0% rather than negative; a non-positive baseline reads 100% when
// nothing is owed now, else 0%.
func TrackProgress(baseline, current int64) Progress {
	if current < 0 {
		current = 0
	}
	paid := baseline - current
	if paid < 0 {
		paid = 0
	}
	pct := 0
	if baseline > 0 {
		pct = int(paid * 100 / baseline)
		if pct > 100 {
			pct = 100
		}
	} else if current == 0 {
		pct = 100
	}
	return Progress{Baseline: baseline, Current: current, PaidOff: paid, Remaining: current, Percent: pct}
}
