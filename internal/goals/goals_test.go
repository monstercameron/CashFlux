// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMonthlyNeeded(t *testing.T) {
	from := mustDate("2026-01-01")

	// $1200 needed over 12 months (target same day-of-month, Jan→Jan next year) → $100/mo.
	g := domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")}
	per, ok, err := MonthlyNeeded(g, from)
	if err != nil || !ok {
		t.Fatalf("expected a projection, ok=%v err=%v", ok, err)
	}
	if per.Amount != 10000 {
		t.Errorf("per month = %d, want 10000 ($100)", per.Amount)
	}

	// No target date → no projection.
	if _, ok, _ := MonthlyNeeded(domain.Goal{TargetAmount: usd(100), TargetDate: time.Time{}}, from); ok {
		t.Error("no target date should give ok=false")
	}
	// Target in the past → no projection.
	past := domain.Goal{TargetAmount: usd(100), TargetDate: mustDate("2025-01-01")}
	if _, ok, _ := MonthlyNeeded(past, from); ok {
		t.Error("past target should give ok=false")
	}
	// Already complete → no projection.
	done := domain.Goal{TargetAmount: usd(100), CurrentAmount: usd(100), TargetDate: mustDate("2027-01-01")}
	if _, ok, _ := MonthlyNeeded(done, from); ok {
		t.Error("complete goal should give ok=false")
	}
}

func TestMonthlyAssignment(t *testing.T) {
	from := mustDate("2026-01-01")
	// Explicit monthly contribution wins — even with no target date (open-ended investing).
	if m, ok, err := MonthlyAssignment(domain.Goal{MonthlyContribution: usd(50000)}, from); err != nil || !ok || m.Amount != 50000 {
		t.Errorf("explicit = %d ok=%v err=%v, want 50000 true", m.Amount, ok, err)
	}
	// No explicit contribution → falls back to the target-date pace.
	dated := domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")} // 12 months → 10000/mo
	if m, ok, _ := MonthlyAssignment(dated, from); !ok || m.Amount != 10000 {
		t.Errorf("dated = %d ok=%v, want 10000 true", m.Amount, ok)
	}
	// Non-financial goals are never assignable, even with a contribution set.
	if _, ok, _ := MonthlyAssignment(domain.Goal{Kind: domain.GoalKindHabit, MonthlyContribution: usd(50000)}, from); ok {
		t.Error("habit goal should not be assignable")
	}
	// No contribution and no target date → not assignable.
	if _, ok, _ := MonthlyAssignment(domain.Goal{TargetAmount: usd(100)}, from); ok {
		t.Error("open goal with no contribution/date should not be assignable")
	}
}

func TestTotalMonthlyAssigned(t *testing.T) {
	from := mustDate("2026-01-01")
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	gs := []domain.Goal{
		{MonthlyContribution: usd(50000)}, // $500 explicit
		{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")}, // $100/mo derived
		{MonthlyContribution: usd(30000), Archived: true},                                      // archived → skipped
		{Kind: domain.GoalKindHabit, MonthlyContribution: usd(90000)},                          // non-financial → skipped
	}
	if got := TotalMonthlyAssigned(gs, from, "USD", rates); got != 60000 {
		t.Errorf("total = %d, want 60000", got)
	}
}

func TestMonthlyNeededRoundsUp(t *testing.T) {
	from := mustDate("2026-01-01")
	// $100 over 3 months → ceil(10000/3) = 3334 minor units.
	g := domain.Goal{TargetAmount: usd(10000), CurrentAmount: usd(0), TargetDate: mustDate("2026-04-01")}
	per, ok, _ := MonthlyNeeded(g, from)
	if !ok || per.Amount != 3334 {
		t.Errorf("per = %d ok=%v, want 3334", per.Amount, ok)
	}
}

func TestMonthlyNeededContributionProjectsToTargetDate(t *testing.T) {
	from := mustDate("2026-01-01")
	g := domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")}
	per, ok, err := MonthlyNeeded(g, from)
	if err != nil || !ok {
		t.Fatalf("MonthlyNeeded ok=%v err=%v", ok, err)
	}
	projected, ok, err := Project(g, per, from)
	if err != nil || !ok {
		t.Fatalf("Project ok=%v err=%v", ok, err)
	}
	// Paying exactly the suggested monthly finishes ON OR BEFORE the deadline —
	// contributions land during the current month first, so the last of the 12
	// payments happens in December, a month inside the Jan 1 target.
	if projected.After(g.TargetDate) {
		t.Errorf("projected = %s lands after the target %s", dateutil.FormatDate(projected), dateutil.FormatDate(g.TargetDate))
	}
}

func goal(target, current int64) domain.Goal {
	return domain.Goal{TargetAmount: usd(target), CurrentAmount: usd(current)}
}

func TestTotals(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	g1 := goal(100000, 30000)
	g2 := goal(50000, 50000)
	g3 := goal(20000, 10000)
	g3.Archived = true

	// Default: archived excluded.
	saved, target := Totals([]domain.Goal{g1, g2, g3}, rates, "USD", false)
	if saved.Amount != 80000 {
		t.Errorf("saved = %d, want 80000 (30000+50000, archived excluded)", saved.Amount)
	}
	if target.Amount != 150000 {
		t.Errorf("target = %d, want 150000 (100000+50000)", target.Amount)
	}
	if saved.Currency != "USD" {
		t.Errorf("saved currency = %q, want USD", saved.Currency)
	}

	// includeArchived folds in g3.
	saved2, target2 := Totals([]domain.Goal{g1, g2, g3}, rates, "USD", true)
	if saved2.Amount != 90000 || target2.Amount != 170000 {
		t.Errorf("with archived: saved=%d target=%d, want 90000/170000", saved2.Amount, target2.Amount)
	}
}

func TestRemaining(t *testing.T) {
	rem, err := Remaining(goal(100000, 30000))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !rem.Equal(usd(70000)) {
		t.Errorf("Remaining = %v, want 70000 USD", rem)
	}
	over, _ := Remaining(goal(100000, 120000))
	if !over.Equal(usd(0)) {
		t.Errorf("Remaining(over) = %v, want 0", over)
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		target, current int64
		want            int
	}{
		{100000, 30000, 30},
		{100000, 0, 0},
		{100000, 120000, 100}, // clamped
		{0, 5000, 100},        // zero target with savings -> complete
		{0, 0, 0},
	}
	for _, tt := range tests {
		if got := Percent(goal(tt.target, tt.current)); got != tt.want {
			t.Errorf("Percent(%d,%d) = %d, want %d", tt.target, tt.current, got, tt.want)
		}
	}
}

func TestRawPercent(t *testing.T) {
	tests := []struct {
		target, current int64
		want            int
	}{
		{100000, 30000, 30},
		{100000, 0, 0},
		{100000, 120000, 120}, // unclamped (overfunded)
		{100000, 100000, 100},
		{0, 5000, 0}, // non-positive target -> 0
		{0, 0, 0},
	}
	for _, tt := range tests {
		if got := RawPercent(goal(tt.target, tt.current)); got != tt.want {
			t.Errorf("RawPercent(%d,%d) = %d, want %d", tt.target, tt.current, got, tt.want)
		}
	}
}

func TestIsComplete(t *testing.T) {
	if c, _ := IsComplete(goal(100000, 100000)); !c {
		t.Error("exactly met should be complete")
	}
	if c, _ := IsComplete(goal(100000, 99999)); c {
		t.Error("under target should not be complete")
	}
}

func TestProject(t *testing.T) {
	from := mustDate("2026-06-15")

	// remaining 60000, monthly 20000 -> 3 payments starting THIS month
	// (Jun, Jul, Aug) -> the last lands 2026-08-15.
	date, ok, err := Project(goal(100000, 40000), usd(20000), from)
	if err != nil || !ok {
		t.Fatalf("Project ok=%v err=%v", ok, err)
	}
	if dateutil.FormatDate(date) != "2026-08-15" {
		t.Errorf("projected = %s, want 2026-08-15", dateutil.FormatDate(date))
	}

	// remaining 65000, monthly 20000 -> ceil(3.25) = 4 payments (Jun..Sep)
	date2, _, _ := Project(goal(100000, 35000), usd(20000), from)
	if dateutil.FormatDate(date2) != "2026-09-15" {
		t.Errorf("projected (ceil) = %s, want 2026-09-15", dateutil.FormatDate(date2))
	}

	// already complete -> from, ok
	dc, okc, _ := Project(goal(100000, 100000), usd(20000), from)
	if !okc || !dc.Equal(from) {
		t.Errorf("complete projection = %v ok=%v, want from/true", dc, okc)
	}

	// non-positive contribution -> no projection
	if _, ok, _ := Project(goal(100000, 0), usd(0), from); ok {
		t.Error("zero contribution should yield no projection")
	}

	// currency mismatch -> error
	if _, _, err := Project(goal(100000, 0), money.New(20000, "EUR"), from); err == nil {
		t.Error("expected currency mismatch error")
	}
}

func TestEvaluate(t *testing.T) {
	from := mustDate("2026-06-15")
	s, err := Evaluate(goal(100000, 40000), usd(20000), from)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s.Percent != 40 || !s.Remaining.Equal(usd(60000)) || s.Complete {
		t.Errorf("status = %+v", s)
	}
	if !s.HasProjection || dateutil.FormatDate(s.Projected) != "2026-08-15" {
		t.Errorf("projection = %v has=%v", s.Projected, s.HasProjection)
	}
}

func TestOverfund(t *testing.T) {
	tests := []struct {
		name    string
		target  int64
		current int64
		want    int64
	}{
		{"exactly at target → 0", 100000, 100000, 0},
		{"over by 20000 → surplus", 100000, 120000, 20000},
		{"under target → 0", 100000, 80000, 0},
		{"zero current → 0", 100000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := goal(tt.target, tt.current)
			got, err := Overfund(g)
			if err != nil {
				t.Fatalf("Overfund error: %v", err)
			}
			if got.Amount != tt.want {
				t.Errorf("Overfund(%d, %d) = %d, want %d", tt.target, tt.current, got.Amount, tt.want)
			}
			// Currency is always preserved from the goal's target currency.
			if got.Currency != "USD" {
				t.Errorf("Overfund currency = %q, want USD", got.Currency)
			}
		})
	}
}

func TestMilestoneCrossed(t *testing.T) {
	tests := []struct {
		name          string
		before        int
		after         int
		wantMilestone int
	}{
		{"no milestone", 20, 24, 0},
		{"exactly at 25", 20, 25, 25},
		{"cross 25", 10, 30, 25},
		{"already past 25", 26, 30, 0},
		{"cross 50", 40, 55, 50},
		{"cross 50 from below 25", 20, 60, 50}, // highest is 50
		{"cross 75", 70, 80, 75},
		{"cross 100", 99, 100, 100},
		{"cross 100 from zero", 0, 100, 100},
		{"before clamped negative", -5, 25, 25},
		{"after clamped above 100", 99, 105, 100},
		{"no change", 50, 50, 0},
		{"decrease", 60, 50, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MilestoneCrossed(tt.before, tt.after)
			if got != tt.wantMilestone {
				t.Errorf("MilestoneCrossed(%d, %d) = %d, want %d", tt.before, tt.after, got, tt.wantMilestone)
			}
		})
	}
}

func TestOverallProgress(t *testing.T) {
	archived := func(target, current int64) domain.Goal {
		g := goal(target, current)
		g.Archived = true
		return g
	}

	tests := []struct {
		name            string
		goals           []domain.Goal
		includeArchived bool
		want            int
	}{
		{"empty → 0", nil, false, 0},
		{"empty include archived → 0", nil, true, 0},
		{"all active, 50%", []domain.Goal{goal(100000, 50000)}, false, 50},
		{"zero target → 0", []domain.Goal{goal(0, 0)}, false, 0},
		{"cap at 100", []domain.Goal{goal(100000, 200000)}, false, 100},
		{
			"archived excluded changes %",
			[]domain.Goal{goal(100000, 50000), archived(100000, 100000)},
			false, // archived goal excluded
			50,    // only the active 50% goal counts
		},
		{
			"archived included",
			[]domain.Goal{goal(100000, 50000), archived(100000, 100000)},
			true,
			75, // (50000+100000)*100 / (100000+100000) = 75
		},
		{
			"multiple active, mixed",
			[]domain.Goal{goal(200000, 100000), goal(100000, 100000)},
			false,
			66, // (100000+100000)*100/300000 = 66 (integer division)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OverallProgress(tt.goals, tt.includeArchived)
			if err != nil {
				t.Fatalf("OverallProgress error: %v", err)
			}
			if got != tt.want {
				t.Errorf("OverallProgress = %d, want %d", got, tt.want)
			}
		})
	}
}
