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
