// SPDX-License-Identifier: MIT

package cashflow

import "time"

// DipDate converts a projection's BreachDay into an absolute calendar date.
// It returns (zero value, false) when the projection never breaches the buffer
// (BreachDay == -1); otherwise it returns (from + BreachDay days, true).
//
// The returned time is midnight UTC on the dip date. Callers that only need to
// check for a breach can use Projection.WillBreach instead.
func DipDate(p Projection, from time.Time) (time.Time, bool) {
	if p.BreachDay < 0 {
		return time.Time{}, false
	}
	return from.AddDate(0, 0, p.BreachDay), true
}

// PaydayBalance returns the projected end-of-day balance at day index `horizon`
// within the projection. The index is clamped so it never falls outside the
// available Daily slice:
//   - If Daily is empty, returns 0.
//   - If horizon >= len(Daily), it is clamped to the last available day.
//   - If the clamped index is still < 0 (horizon was negative), returns 0.
//
// This is typically called with the payday horizon from NextPaydayHorizon, which
// may have been computed against a different (longer) projection window, so
// bounds-guarding is essential.
func PaydayBalance(p Projection, horizon int) int64 {
	if len(p.Daily) == 0 {
		return 0
	}
	idx := horizon
	if idx >= len(p.Daily) {
		idx = len(p.Daily) - 1
	}
	if idx < 0 {
		return 0
	}
	return p.Daily[idx].Balance
}
