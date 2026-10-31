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
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	// ErrInvalidHistory reports malformed stored contribution history.
	ErrInvalidHistory = errors.New("goalhistory: invalid contribution history")
	// ErrCurrencyMismatch reports history whose amounts cannot be shown in the
	// goal's current target currency without an explicit conversion rate.
	ErrCurrencyMismatch = errors.New("goalhistory: contribution currency mismatch")
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

// ShortfallMinor is the compatibility wrapper for ShortfallChecked. It returns
// zero if the shortfall cannot fit; financial readers should use the checked
// method when that distinction matters.
func (p Point) ShortfallMinor() int64 {
	v, _ := p.ShortfallChecked()
	return v
}

// ShortfallChecked returns the positive gap between plan and actual, or an
// overflow error when that gap cannot fit in int64.
func (p Point) ShortfallChecked() (int64, error) {
	if p.PlannedMinor <= 0 || p.ActualMinor >= p.PlannedMinor {
		return 0, nil
	}
	return money.Sub(p.PlannedMinor, p.ActualMinor)
}

// Series is the compatibility wrapper for SeriesChecked. It returns nil when
// stored history is invalid or its sums overflow; financial readers should use
// SeriesChecked to distinguish unavailable history from an empty one.
//
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
	pts, err := SeriesChecked(g, plannedMinor, months, now)
	if err != nil {
		return nil
	}
	return pts
}

// SeriesChecked is Series with explicit validation and overflow errors. It
// returns no partial series. Contributions in the displayed window must use
// the goal's target currency because this read model has no FX table.
func SeriesChecked(g domain.Goal, plannedMinor int64, months int, now time.Time) ([]Point, error) {
	if months <= 0 {
		return nil, nil
	}
	type bucket struct {
		amount   int64
		count    int
		detailed bool
	}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	inWindow := func(key string) bool { return key >= start.Format("2006-01") && key < end.Format("2006-01") }
	buckets := map[string]*bucket{}
	expectedCurrency := g.TargetAmount.Currency
	add := func(key, currency string, amount int64, count int, detailed bool) error {
		if _, err := time.Parse("2006-01", key); err != nil {
			return fmt.Errorf("%w: goal %q has invalid month %q", ErrInvalidHistory, g.ID, key)
		}
		if !inWindow(key) {
			return nil
		}
		if currency == "" {
			return fmt.Errorf("%w: goal %q month %s has no currency", ErrInvalidHistory, g.ID, key)
		}
		if expectedCurrency == "" {
			expectedCurrency = currency // legacy test/import records may omit the goal target currency
		}
		if currency != expectedCurrency {
			return fmt.Errorf("%w: goal %q month %s has %s, expected %s", ErrCurrencyMismatch, g.ID, key, currency, expectedCurrency)
		}
		if amount < 0 || count < 0 {
			return fmt.Errorf("%w: goal %q month %s has negative amount or count", ErrInvalidHistory, g.ID, key)
		}
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		total, err := money.Add(b.amount, amount)
		if err != nil {
			return fmt.Errorf("goalhistory: goal %q month %s amount: %w", g.ID, key, err)
		}
		if count > math.MaxInt-b.count {
			return fmt.Errorf("goalhistory: goal %q month %s count: %w", g.ID, key, money.ErrOverflow)
		}
		b.amount, b.count, b.detailed = total, b.count+count, b.detailed || detailed
		return nil
	}
	// Roll-ups first; the itemized entries add on top.
	for _, h := range g.ContributionHistory {
		if err := add(h.Month, h.Amount.Currency, h.Amount.Amount, h.Count, false); err != nil {
			return nil, err
		}
	}
	for _, c := range g.Contributions {
		k := MonthKey(c.At)
		if err := add(k, c.Amount.Currency, c.Amount.Amount, 1, true); err != nil {
			return nil, err
		}
	}

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
	return out, nil
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

// Summarize is the compatibility wrapper for SummarizeChecked. It returns an
// empty summary on overflow; financial readers should use SummarizeChecked to
// distinguish unavailable totals from valid zeroes.
func Summarize(pts []Point) Summary {
	s, _ := SummarizeChecked(pts)
	return s
}

// SummarizeChecked reduces a series to checked totals. On error it returns an
// empty summary rather than a partial or wrapped financial total.
func SummarizeChecked(pts []Point) (Summary, error) {
	s := Summary{Months: len(pts)}
	for _, p := range pts {
		var err error
		s.ActualMinor, err = money.Add(s.ActualMinor, p.ActualMinor)
		if err != nil {
			return Summary{}, fmt.Errorf("goalhistory: actual total: %w", err)
		}
		s.PlannedMinor, err = money.Add(s.PlannedMinor, p.PlannedMinor)
		if err != nil {
			return Summary{}, fmt.Errorf("goalhistory: planned total: %w", err)
		}
		if p.PlannedMinor <= 0 {
			continue
		}
		if p.Met() {
			s.FundedMonths++
		} else {
			s.MissedMonths++
			shortfall, err := p.ShortfallChecked()
			if err != nil {
				return Summary{}, fmt.Errorf("goalhistory: monthly shortfall: %w", err)
			}
			s.ShortfallMinor, err = money.Add(s.ShortfallMinor, shortfall)
			if err != nil {
				return Summary{}, fmt.Errorf("goalhistory: shortfall total: %w", err)
			}
		}
	}
	return s, nil
}

// PercentOfPeak returns value as an integer percentage of peak, using a wide
// intermediate so chart widths remain correct near int64 limits.
func PercentOfPeak(value, peak int64) (int, error) {
	if value < 0 || peak < 0 {
		return 0, fmt.Errorf("%w: negative chart amount", ErrInvalidHistory)
	}
	if value == 0 || peak == 0 {
		return 0, nil
	}
	pct, err := money.MulDiv(value, 100, peak, money.RoundTowardZero)
	if err != nil {
		return 0, fmt.Errorf("goalhistory: chart percentage: %w", err)
	}
	return int(min(pct, 100)), nil
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
