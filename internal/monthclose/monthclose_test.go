// SPDX-License-Identifier: MIT

package monthclose

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func status(id, name string, remaining int64) budgeting.Status {
	return budgeting.Status{
		Budget:    domain.Budget{ID: id, Name: name},
		Remaining: money.New(remaining, "USD"),
	}
}

func TestBuildSplitsAndSorts(t *testing.T) {
	sts := []budgeting.Status{
		status("b1", "Groceries", -2500),
		status("b2", "Dining", 1000),
		status("b3", "Fuel", -7500),
		status("b4", "Fun", 4000),
		status("b5", "Exact", 0),
	}
	s := Build(sts, nil, 500000, 480000, 12000, false)

	if len(s.Overspends) != 2 || len(s.Leftovers) != 2 {
		t.Fatalf("split = %d over / %d left, want 2/2", len(s.Overspends), len(s.Leftovers))
	}
	if s.Overspends[0].BudgetID != "b3" || s.Overspends[0].Minor != 7500 {
		t.Errorf("largest overage first: got %+v", s.Overspends[0])
	}
	if s.Leftovers[0].BudgetID != "b4" || s.Leftovers[0].Minor != 4000 {
		t.Errorf("largest leftover first: got %+v", s.Leftovers[0])
	}
	if s.TotalOverMinor != 10000 || s.TotalLeftMinor != 5000 {
		t.Errorf("totals = %d over / %d left, want 10000/5000", s.TotalOverMinor, s.TotalLeftMinor)
	}
	if got := s.IncomeDeltaMinor(); got != -20000 {
		t.Errorf("income delta = %d, want -20000", got)
	}
	if s.Clean() {
		t.Error("summary with overspends + over-assignment must not be Clean")
	}
}

func TestBuildTiesSortByName(t *testing.T) {
	s := Build([]budgeting.Status{
		status("bz", "Zeta", 300),
		status("ba", "Alpha", 300),
	}, nil, 0, 0, 0, false)
	if s.Leftovers[0].Name != "Alpha" {
		t.Errorf("equal amounts sort by name: got %q first", s.Leftovers[0].Name)
	}
}

func TestBuildNameOfFallback(t *testing.T) {
	nameOf := func(b domain.Budget) string {
		if b.ID == "b1" {
			return "Groceries (Food)"
		}
		return ""
	}
	s := Build([]budgeting.Status{
		status("b1", "raw1", -100),
		status("b2", "raw2", -100),
	}, nameOf, 0, 0, 0, false)
	if s.Overspends[0].Name != "Groceries (Food)" && s.Overspends[1].Name != "Groceries (Food)" {
		t.Error("nameOf result not used")
	}
	for _, it := range s.Overspends {
		if it.BudgetID == "b2" && it.Name != "raw2" {
			t.Errorf("empty nameOf must fall back to Budget.Name, got %q", it.Name)
		}
	}
}

func TestBuildClampsNegativeOverAssigned(t *testing.T) {
	s := Build(nil, nil, 0, 0, -500, false)
	if s.OverAssignedMinor != 0 {
		t.Errorf("negative over-assignment must clamp to 0, got %d", s.OverAssignedMinor)
	}
	if !s.Clean() {
		t.Error("empty summary must be Clean")
	}
}

func TestResolutions(t *testing.T) {
	cases := []struct {
		name string
		s    Summary
		want []string
	}{
		{"not over-assigned", Summary{}, nil},
		{
			"leftovers + rollover off",
			Summary{OverAssignedMinor: 100, Leftovers: []Item{{Minor: 50}}, TotalLeftMinor: 50},
			[]string{ResolveReduce, ResolveIncome, ResolveRollover, ResolveDefer},
		},
		{
			"no leftovers",
			Summary{OverAssignedMinor: 100},
			[]string{ResolveIncome, ResolveDefer},
		},
		{
			"rollover already on",
			Summary{OverAssignedMinor: 100, Leftovers: []Item{{Minor: 50}}, TotalLeftMinor: 50, RolloverOn: true},
			[]string{ResolveReduce, ResolveIncome, ResolveDefer},
		},
	}
	for _, tc := range cases {
		got := Resolutions(tc.s)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

func TestCopyBoosts(t *testing.T) {
	last := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	this := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	boosted := domain.Budget{ID: "b1"}.WithPeriodBoost(last, 5000)
	unboosted := domain.Budget{ID: "b2"}
	alreadyThis := domain.Budget{ID: "b3"}.WithPeriodBoost(last, 2000).WithPeriodBoost(this, 900)
	excluded := domain.Budget{ID: "b4"}.WithPeriodBoost(last, 1500)

	starts := func(domain.Budget) (time.Time, time.Time) { return last, this }
	got := CopyBoosts([]domain.Budget{boosted, unboosted, alreadyThis, excluded}, starts, map[string]bool{"b4": true})

	if len(got) != 1 {
		t.Fatalf("plan size = %d, want 1 (only b1): %v", len(got), got)
	}
	if got["b1"] != 5000 {
		t.Errorf("b1 boost = %d, want 5000", got["b1"])
	}
}

// TestCarryTargets pins the DIRECTION of the carry. The regression this guards
// is a silent one: the old caller derived (last, this), so at a period's end the
// plan wrote the PREVIOUS period's top-ups into the period that was ending. No
// error, no empty result — just boosts landing where they could never be spent.
func TestCarryTargets(t *testing.T) {
	ws := time.Sunday
	cases := []struct {
		name         string
		period       domain.Period
		ref          time.Time
		source, want time.Time
	}{
		{
			// The exact shape of the bug: reviewing August on its final day.
			name:   "monthly on the last day carries august into september",
			period: domain.PeriodMonthly,
			ref:    time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
			source: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			want:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// A closed period reviewed from the next one anchors to the window start;
			// the target must still be the period AFTER the one under review.
			name:   "monthly anchored to the window start still moves forward",
			period: domain.PeriodMonthly,
			ref:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			source: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			want:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// Year boundary: the target is next January, not month 13.
			name:   "monthly crosses the year boundary",
			period: domain.PeriodMonthly,
			ref:    time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
			source: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			want:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// Budgets can run weekly, so the target has to follow the budget's own
			// cadence rather than the calendar month the modal is titled after.
			name:   "weekly advances by one week",
			period: domain.PeriodWeekly,
			ref:    time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), // Thursday
			source: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), // Sunday
			want:   time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "quarterly advances by one quarter",
			period: domain.PeriodQuarterly,
			ref:    time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
			source: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			want:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		src, tgt := CarryTargets(tc.period, tc.ref, ws)
		if !src.Equal(tc.source) {
			t.Errorf("%s: source = %s, want %s", tc.name, src.Format("2006-01-02"), tc.source.Format("2006-01-02"))
		}
		if !tgt.Equal(tc.want) {
			t.Errorf("%s: target = %s, want %s", tc.name, tgt.Format("2006-01-02"), tc.want.Format("2006-01-02"))
		}
		if !tgt.After(src) {
			t.Errorf("%s: target %s must be after source %s — a carry never goes backwards",
				tc.name, tgt.Format("2006-01-02"), src.Format("2006-01-02"))
		}
	}
}

// TestCopyBoostsCarriesForward wires CarryTargets into CopyBoosts the way the
// modal does, so the two are pinned together rather than only in isolation: the
// plan must read the reviewed period's boost and key it to the NEXT period.
func TestCopyBoostsCarriesForward(t *testing.T) {
	ws := time.Sunday
	ref := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	aug := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	sep := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	jul := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Boosted in August (the period under review) — must carry.
	carried := domain.Budget{ID: "carry", Period: domain.PeriodMonthly}.WithPeriodBoost(aug, 4200)
	// Boosted only in JULY. Under the old (last, this) direction this was the one
	// that copied; carrying forward from August it must NOT.
	stale := domain.Budget{ID: "stale", Period: domain.PeriodMonthly}.WithPeriodBoost(jul, 9900)
	// Already has a September top-up set by hand — copying must never stack on it.
	manual := domain.Budget{ID: "manual", Period: domain.PeriodMonthly}.
		WithPeriodBoost(aug, 1000).WithPeriodBoost(sep, 250)

	starts := func(b domain.Budget) (time.Time, time.Time) { return CarryTargets(b.Period, ref, ws) }
	got := CopyBoosts([]domain.Budget{carried, stale, manual}, starts, nil)

	if len(got) != 1 {
		t.Fatalf("plan = %v, want only {carry:4200}", got)
	}
	if got["carry"] != 4200 {
		t.Errorf("carry = %d, want 4200 (August's boost carried into September)", got["carry"])
	}
	if _, ok := got["stale"]; ok {
		t.Error("stale carried a JULY boost — the carry direction has regressed to (last, this)")
	}
	if _, ok := got["manual"]; ok {
		t.Error("manual already has a September boost; copying must not stack on it")
	}
}
