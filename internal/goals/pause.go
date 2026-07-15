// SPDX-License-Identifier: MIT

package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// PauseCost is the honest schedule impact of pausing a goal for a number of
// months (GL7): the finish date before the pause, the finish date after it, and
// the delta in months between them. It is what the "Pause goal" confirm shows
// BEFORE committing ("pausing 2 months moves the finish to March"), so the user
// chooses the pause with its cost in full view rather than discovering it later.
type PauseCost struct {
	// Months is the length of the pause being previewed.
	Months int
	// Original is the finish date the goal is on course for without pausing —
	// either its projected completion (at the assumed monthly rate) or, failing
	// that, its target date. Valid only when HasFinish is true.
	Original time.Time
	// Shifted is Original moved out by Months — the finish the pause implies.
	// Valid only when HasFinish is true.
	Shifted time.Time
	// HasFinish reports whether a finish date is known to show. It is false for a
	// goal with neither a usable projection nor a target date, in which case the
	// pause has no datable cost to preview (the goal simply resumes later).
	HasFinish bool
}

// ComputePauseCost projects how pausing a goal for the given number of months
// shifts its finish date, evaluated from the reference time `from`. It is pure
// and deterministic. The base finish is the goal's projected completion at the
// assumed monthly contribution when that is available; otherwise it falls back
// to the goal's target date. The shifted finish is the base moved out by the
// pause length, since contributions resume at the same rate afterwards. A
// non-positive months value yields a zero-cost preview (Original == Shifted).
func ComputePauseCost(goal domain.Goal, monthly money.Money, from time.Time, months int) (PauseCost, error) {
	if months < 0 {
		months = 0
	}
	cost := PauseCost{Months: months}
	base, ok, err := Project(goal, monthly, from)
	if err != nil {
		return PauseCost{}, err
	}
	if !ok {
		// No projection (no usable monthly rate): fall back to the target date so a
		// dated goal still shows an honest shift.
		if goal.TargetDate.IsZero() {
			return cost, nil // nothing datable to preview
		}
		base = goal.TargetDate
	}
	cost.Original = base
	cost.Shifted = dateutil.AddMonths(base, months)
	cost.HasFinish = true
	return cost, nil
}

// PausedUntilFrom returns the date a pause of the given number of months, begun
// at `from`, runs until — the value to store in domain.Goal.PausedUntil. A
// non-positive months value returns the zero time (no pause). It is the single
// place the pause-end date is derived, so the UI and any read-model agree.
func PausedUntilFrom(from time.Time, months int) time.Time {
	if months <= 0 {
		return time.Time{}
	}
	return dateutil.AddMonths(from, months)
}
