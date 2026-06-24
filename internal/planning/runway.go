// SPDX-License-Identifier: MIT

package planning

import (
	"github.com/monstercameron/CashFlux/internal/domain"
)

// RunwayMonths returns how many months the plan's starting balance lasts before
// the projected balance first crosses below zero, using linear interpolation
// within the crossing month.
//
// depletes is true when the balance crosses below zero within the plan's
// horizon. When depletes is false the balance stays non-negative throughout and
// months is 0 (the caller should show a "stays positive" indicator instead).
//
// The interpolation is linear across each month boundary:
//
//	fraction = prev / (prev − cur)   (float64; both values converted from int64)
//	months   = (i−1) + fraction      (i is the 1-based month index that first goes negative)
//
// One-time events land at month boundaries, so the fraction across a month that
// contains a one-time dip is a linear approximation — deterministic and
// consistent with the displayed monthly curve, but not calendar-exact.
//
// Edge cases:
//   - StartBalance ≤ 0 with a non-increasing trajectory → (0, true).
//   - Balance hits exactly zero → not considered depleted (depletes = false).
//   - HorizonMonths ≤ 0 → (0, false).
func RunwayMonths(p domain.Plan) (months float64, depletes bool) {
	if p.HorizonMonths <= 0 {
		return 0, false
	}

	curve := Project(p) // end-of-month balances, len == HorizonMonths

	// Treat the starting balance as the balance at month 0 (before any flows).
	prev := p.StartBalance

	// Special case: already at or below zero with nothing going up.
	if prev <= 0 {
		// If the first projected month is also ≤ prev (non-increasing), deplete immediately.
		if len(curve) == 0 || curve[0] <= prev {
			return 0, true
		}
	}

	for i, cur := range curve {
		if cur < 0 {
			// Month i+1 (1-based) is the crossing month.
			// prev is the balance entering this month (end of month i, or StartBalance when i==0).
			var frac float64
			denom := float64(prev) - float64(cur) // prev − cur; always > 0 since prev ≥ 0 > cur
			if denom > 0 {
				frac = float64(prev) / denom
			}
			// clamp fraction to [0, 1] for safety (degenerate input guard)
			if frac < 0 {
				frac = 0
			} else if frac > 1 {
				frac = 1
			}
			return float64(i) + frac, true
		}
		prev = cur
	}

	return 0, false
}
