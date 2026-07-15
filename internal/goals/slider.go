// SPDX-License-Identifier: MIT

package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SliderPoint is one position on the contribution slider (GL4): a candidate
// monthly contribution and the finish date it implies for the goal. HasFinish is
// false when no finish can be projected at this amount (a non-positive
// contribution on an unmet goal), so the UI can grey that end of the track.
type SliderPoint struct {
	// Monthly is the candidate monthly contribution.
	Monthly money.Money
	// Finish is the projected completion date at Monthly (valid only if HasFinish).
	Finish time.Time
	// HasFinish reports whether Finish is meaningful.
	HasFinish bool
	// OnTrack reports whether, at Monthly, the goal is projected to finish on or
	// before its TargetDate. Meaningful only when the goal has a target date and a
	// finish could be projected (HasFinish); otherwise false.
	OnTrack bool
}

// SliderRange proposes a sensible min/max/step for the contribution slider for a
// goal, evaluated at `from`. The range is anchored on the pace the goal needs to
// hit its target date (MonthlyNeeded) when it has one, else on its explicit
// MonthlyContribution, else on a fraction of the remaining balance — always
// giving the user a band that brackets a realistic plan. All three results share
// the goal's currency. ok is false when there is nothing to size a range against
// (no remaining balance).
func SliderRange(goal domain.Goal, from time.Time) (min, max, step money.Money, ok bool) {
	rem, err := Remaining(goal)
	if err != nil || rem.Amount <= 0 {
		return money.Money{}, money.Money{}, money.Money{}, false
	}
	cur := goal.TargetAmount.Currency

	// Anchor: the most informative monthly figure the goal already implies.
	var anchor int64
	if needed, has, err := MonthlyNeeded(goal, from); err == nil && has && needed.Amount > 0 {
		anchor = needed.Amount
	} else if goal.MonthlyContribution.Amount > 0 {
		anchor = goal.MonthlyContribution.Amount
	} else {
		anchor = rem.Amount / 12 // a "one year" default pace
	}
	if anchor <= 0 {
		anchor = rem.Amount // degenerate: pay it in one month
	}

	// Bracket the anchor generously so the user can explore both faster and slower
	// plans; cap the top at the whole remaining balance (finishing next month).
	lo := anchor / 4
	if lo <= 0 {
		lo = 1
	}
	hi := anchor * 4
	if hi > rem.Amount {
		hi = rem.Amount
	}
	if hi <= lo {
		hi = lo * 2
	}
	st := roundStep((hi - lo) / 20)
	return money.New(lo, cur), money.New(hi, cur), money.New(st, cur), true
}

// SliderPointAt projects the finish date for one candidate monthly contribution
// — the live read-model the slider calls as the user drags. It is a thin,
// currency-checked wrapper over Project plus the on-track judgement.
func SliderPointAt(goal domain.Goal, monthlyMinor int64, from time.Time) SliderPoint {
	m := money.New(monthlyMinor, goal.TargetAmount.Currency)
	pt := SliderPoint{Monthly: m}
	finish, has, err := Project(goal, m, from)
	if err != nil || !has {
		return pt
	}
	pt.Finish, pt.HasFinish = finish, true
	if !goal.TargetDate.IsZero() {
		pt.OnTrack = !finish.After(goal.TargetDate)
	}
	return pt
}

// SliderTicks evaluates a set of candidate monthly contributions into finish
// dates in one pass — used to render the discrete stops (and the "$150/mo → Aug
// 2027; $250/mo → Nov 2026" preview) without the UI looping over Project itself.
func SliderTicks(goal domain.Goal, monthlyMinors []int64, from time.Time) []SliderPoint {
	out := make([]SliderPoint, 0, len(monthlyMinors))
	for _, m := range monthlyMinors {
		out = append(out, SliderPointAt(goal, m, from))
	}
	return out
}

// roundStep rounds a raw step size to a friendly increment (whole dollars where
// possible) so the slider snaps to readable amounts rather than odd cents.
func roundStep(raw int64) int64 {
	switch {
	case raw <= 0:
		return 1
	case raw < 100:
		return raw // sub-dollar goals: keep the fine step
	case raw < 1000: // under $10: snap to the nearest dollar
		return (raw / 100) * 100
	default: // snap to the nearest $5
		return (raw / 500) * 500
	}
}
