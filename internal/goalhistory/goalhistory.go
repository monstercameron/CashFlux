// SPDX-License-Identifier: MIT

// Package goalhistory turns a goal's contribution record into a month-by-month
// series of what was planned against what actually landed (C399).
//
// The goal card answers "am I on track" with a single projected date. That
// answer is only as good as the assumption underneath it — that the monthly plan
// is what actually gets contributed — and nothing on the surface ever tested the
// assumption. A goal whose owner has quietly funded half the plan for six months
// still shows a confident date computed from the full plan.
//
// The series is assembled from two sources: the detailed contribution log
// (recent, itemized, supports undo) and the rolled-up monthly totals of
// everything that aged out of it. They SUM, and a month routinely appears in
// both — the month the log filled up has some of its contributions rolled up and
// the rest still itemized. Double-counting is impossible by construction rather
// than by a rule here: a contribution is rolled up exactly when it leaves the
// log, so it is in one place or the other, never both.
package goalhistory

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// MonthKey formats a time as the calendar-month key the series and the stored
// roll-ups both use.
func MonthKey(t time.Time) string { return t.Format("2006-01") }

// Point is one month of the series.
type Point struct {
	// Month is the calendar month as "2006-01".
	Month string
	// Label is the month as "Jan 2026", for an axis.
	Label string
	// ActualMinor is what was contributed that month, in minor units.
	ActualMinor int64
	// PlannedMinor is the monthly plan for that month. It is the CURRENT plan
	// applied backwards, because the app does not keep a history of plan changes
	// — see Series for why that is stated rather than hidden.
	PlannedMinor int64
	// Count is how many contributions the month holds.
	Count int
	// Detailed is true when the month holds at least one itemized contribution,
	// so a UI can offer per-contribution drill-down only where there is something
	// to drill into. A partly-rolled-up month is still detailed — some of its
	// entries can be listed.
	Detailed bool
}

// Met reports whether the month's contributions reached its plan. A month with
// no plan is trivially met — there was nothing to fall short of.
func (p Point) Met() bool { return p.PlannedMinor <= 0 || p.ActualMinor >= p.PlannedMinor }

// ShortfallMinor is how far under plan the month came, or 0 when it met or beat
// it. Never negative: an overshoot is not a negative shortfall, and callers that
// sum shortfalls must not have a good month cancel a bad one.
func (p Point) ShortfallMinor() int64 {
	if p.PlannedMinor <= 0 || p.ActualMinor >= p.PlannedMinor {
		return 0
	}
	return p.PlannedMinor - p.ActualMinor
}

// Series returns the last months calendar months ending with the month
// containing now, oldest first, with no gaps — a month in which nothing was
// contributed is a zero point, not an absent one, because the gap IS the finding
// and dropping it would draw a chart of only the good months.
//
// plannedMinor is the goal's current monthly plan, applied to every month. The
// app stores no history of plan changes, so this is an approximation; it is
// applied openly rather than hidden because the alternative — showing actuals
// with no reference line — leaves the reader with numbers and no question.
// A non-positive plannedMinor produces a series with no planned line at all.
//
// A months of zero or less returns nil.
func Series(g domain.Goal, plannedMinor int64, months int, now time.Time) []Point {
	if months <= 0 {
		return nil
	}
	type bucket struct {
		amount   int64
		count    int
		detailed bool
	}
	buckets := map[string]*bucket{}
	// Roll-ups first; the itemized entries add on top.
	for _, h := range g.ContributionHistory {
		buckets[h.Month] = &bucket{amount: h.Amount.Amount, count: h.Count}
	}
	for _, c := range g.Contributions {
		k := MonthKey(c.At)
		b, ok := buckets[k]
		if !ok {
			b = &bucket{}
			buckets[k] = b
		}
		// Adds to whatever the roll-up already holds for this month. The month the
		// log filled up has some entries rolled up and some still itemized, and
		// only the sum is that month's real funding.
		b.amount += c.Amount.Amount
		b.count++
		b.detailed = true
	}

	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)
	out := make([]Point, 0, months)
	for i := range months {
		m := start.AddDate(0, i, 0)
		k := MonthKey(m)
		p := Point{Month: k, Label: m.Format("Jan 2006"), PlannedMinor: max64(plannedMinor, 0)}
		if b, ok := buckets[k]; ok {
			p.ActualMinor, p.Count, p.Detailed = b.amount, b.count, b.detailed
		}
		out = append(out, p)
	}
	return out
}

// Summary is the read over a whole series — the sentence the panel needs so a
// reader does not have to add up twelve bars themselves.
type Summary struct {
	// Months is how many months the series covers.
	Months int
	// FundedMonths is how many of them met or beat the plan.
	FundedMonths int
	// MissedMonths is how many fell short of a plan that existed. Months with no
	// plan count as neither: there was nothing to hit or miss.
	MissedMonths int
	// ActualMinor and PlannedMinor are the totals across the window.
	ActualMinor, PlannedMinor int64
	// ShortfallMinor is the sum of the per-month shortfalls, which is NOT
	// PlannedMinor minus ActualMinor: one generous month must not erase five
	// thin ones, because the plan is a monthly commitment and catching up later
	// does not put the earlier months back.
	ShortfallMinor int64
}

// OnPlan reports whether every month with a plan met it.
func (s Summary) OnPlan() bool { return s.MissedMonths == 0 }

// Summarize reduces a series to its totals.
func Summarize(pts []Point) Summary {
	s := Summary{Months: len(pts)}
	for _, p := range pts {
		s.ActualMinor += p.ActualMinor
		s.PlannedMinor += p.PlannedMinor
		if p.PlannedMinor <= 0 {
			continue
		}
		if p.Met() {
			s.FundedMonths++
		} else {
			s.MissedMonths++
			s.ShortfallMinor += p.ShortfallMinor()
		}
	}
	return s
}

// PeakMinor is the largest of the actual and planned values across the series —
// the denominator a bar chart needs so both series share one scale. Zero when
// the series is empty or entirely zero, which callers must treat as "nothing to
// draw" rather than dividing by it.
func PeakMinor(pts []Point) int64 {
	var peak int64
	for _, p := range pts {
		peak = max64(peak, max64(p.ActualMinor, p.PlannedMinor))
	}
	return peak
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
