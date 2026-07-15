// SPDX-License-Identifier: MIT

// Package smoothing implements annual/quarterly bill smoothing (sinking-fund
// accrual) for recurring cash flows (XC3). A large yearly premium that lands in
// one budget period is really a small monthly cost of living; smoothing spreads
// it so the off periods accrue a virtual monthly set-aside and the landing period
// reads roughly on-pace instead of a large one-period blowout.
//
// The package is pure (no syscall/js): it computes the per-month accrual, detects
// which periods a bill lands in, and identifies the system-managed sinking-fund
// goal that holds the set-aside. Persistence, goal creation, and UI live in the
// layers above.
package smoothing

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// GoalCustomKey is the Goal.Custom entry that marks a goal as the system-managed
// sinking fund for a smoothed recurring. Its value is the owning recurring's ID.
// Readers use IsSmoothingGoal / SmoothingGoalFor rather than the raw key.
const GoalCustomKey = "smoothingRecurringID"

// MonthlyAccrual returns the virtual per-month set-aside (a positive magnitude in
// minor units) for a smoothed recurring: a yearly amount divided across twelve
// months, a quarterly amount across three. It returns 0 when the recurring does
// not smooth (flag off, or a cadence with no off periods to accrue across).
func MonthlyAccrual(r domain.Recurring) int64 {
	if !r.Smooths() {
		return 0
	}
	eq := r.MonthlyEquivalent()
	if eq < 0 {
		eq = -eq
	}
	return eq
}

// prevDue returns the due date one cadence step BEFORE from — the inverse of
// domain.RecurringCadence.Next, used to walk a schedule backwards to the start of
// a period window.
func prevDue(c domain.RecurringCadence, from time.Time) time.Time {
	switch c {
	case domain.CadenceDaily:
		return from.AddDate(0, 0, -1)
	case domain.CadenceWeekly:
		return from.AddDate(0, 0, -7)
	case domain.CadenceBiweekly:
		return from.AddDate(0, 0, -14)
	case domain.CadenceSemimonthly:
		return dateutil.AddMonths(from, -1)
	case domain.CadenceQuarterly:
		return dateutil.AddMonths(from, -3)
	case domain.CadenceYearly:
		return dateutil.AddMonths(from, -12)
	default: // monthly and unknown
		return dateutil.AddMonths(from, -1)
	}
}

// OccurrencesIn returns the recurring's due dates that fall within the half-open
// window [start, end), ordered ascending. It walks the schedule backward from
// NextDue to the window start, then forward, so occurrences on either side of
// NextDue are found. A degenerate or empty window returns nil.
func OccurrencesIn(r domain.Recurring, start, end time.Time) []time.Time {
	if !end.After(start) || r.NextDue.IsZero() || r.Cadence == "" {
		return nil
	}
	d := r.NextDue
	// Walk back until strictly before the window start.
	for guard := 0; !d.Before(start) && guard < 4000; guard++ {
		p := prevDue(r.Cadence, d)
		if !p.Before(d) {
			break // no progress — avoid an infinite loop
		}
		d = p
	}
	var out []time.Time
	for guard := 0; d.Before(end) && guard < 4000; guard++ {
		if !d.Before(start) {
			out = append(out, d)
		}
		d = r.Cadence.Next(d)
	}
	return out
}

// LandsIn reports whether a smoothed recurring's bill lands within [start, end) —
// the "landing period" whose posted spend the accrued fund offsets. Non-smoothed
// recurrings never land (they are not smoothed at all).
func LandsIn(r domain.Recurring, start, end time.Time) bool {
	if !r.Smooths() {
		return false
	}
	return len(OccurrencesIn(r, start, end)) > 0
}

// IsSmoothingGoal reports whether a goal is a system-managed sinking fund created
// for a smoothed recurring (it carries the GoalCustomKey marker).
func IsSmoothingGoal(g domain.Goal) bool {
	if g.Custom == nil {
		return false
	}
	v, ok := g.Custom[GoalCustomKey]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && s != ""
}

// SmoothingRecurringID returns the recurring ID a system-managed sinking-fund goal
// belongs to, or "" when the goal is not a smoothing goal.
func SmoothingRecurringID(g domain.Goal) string {
	if g.Custom == nil {
		return ""
	}
	if s, ok := g.Custom[GoalCustomKey].(string); ok {
		return s
	}
	return ""
}

// SmoothingGoalFor returns the system-managed sinking-fund goal owned by
// recurringID, and true when one exists among goals.
func SmoothingGoalFor(goals []domain.Goal, recurringID string) (domain.Goal, bool) {
	for _, g := range goals {
		if SmoothingRecurringID(g) == recurringID {
			return g, true
		}
	}
	return domain.Goal{}, false
}
