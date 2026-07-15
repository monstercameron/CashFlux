// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestMonthsElapsed(t *testing.T) {
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		since time.Time
		now   time.Time
		want  int
	}{
		{"zero since", time.Time{}, base, 0},
		{"future since", base.AddDate(0, 1, 0), base, 0},
		{"exactly 3 months", base, base.AddDate(0, 3, 0), 3},
		{"partial month", base, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MonthsElapsed(c.since, c.now); got != c.want {
				t.Fatalf("MonthsElapsed = %d, want %d", got, c.want)
			}
		})
	}
}

func TestBuildPledgeReadout(t *testing.T) {
	g := domain.Goal{
		ID:           "g1",
		Name:         "House",
		TargetAmount: money.New(5000000, "USD"),
		Pledges: map[string]money.Money{
			"you":   money.New(20000, "USD"), // $200/mo
			"priya": money.New(20000, "USD"),
		},
		Contributions: []domain.GoalContribution{
			{Amount: money.New(60000, "USD"), MemberID: "you", At: time.Now()},   // $600 over 2 months → $200 ahead
			{Amount: money.New(40000, "USD"), MemberID: "priya", At: time.Now()}, // $400 → on pace
		},
	}
	r := BuildPledgeReadout(g, "", 2) // 2 months elapsed

	if len(r.Standings) != 2 {
		t.Fatalf("want 2 standings, got %d", len(r.Standings))
	}
	if r.TotalPledged.Amount != 40000 {
		t.Fatalf("total pledged = %d", r.TotalPledged.Amount)
	}
	if r.TotalActual.Amount != 100000 {
		t.Fatalf("total actual = %d", r.TotalActual.Amount)
	}
	byMember := map[string]PledgeStanding{}
	for _, s := range r.Standings {
		byMember[s.MemberID] = s
	}
	you := byMember["you"]
	if you.ExpectedToDate.Amount != 40000 { // 200*2
		t.Fatalf("you expected = %d", you.ExpectedToDate.Amount)
	}
	if you.Delta.Amount != 20000 { // 600-400
		t.Fatalf("you delta = %d, want 20000", you.Delta.Amount)
	}
	if you.AheadMonths != 1 {
		t.Fatalf("you aheadMonths = %d, want 1", you.AheadMonths)
	}
	if you.Pace() != PledgePaceOnPace {
		// delta 20000 is exactly one month's pledge → within tolerance → on pace
		t.Fatalf("you pace = %q, want onpace", you.Pace())
	}
	priya := byMember["priya"]
	if priya.Delta.Amount != 0 || priya.Pace() != PledgePaceOnPace {
		t.Fatalf("priya delta=%d pace=%q", priya.Delta.Amount, priya.Pace())
	}
}

func TestBuildPledgeReadoutPaceThresholds(t *testing.T) {
	g := domain.Goal{
		TargetAmount: money.New(1000000, "USD"),
		Pledges:      map[string]money.Money{"a": money.New(10000, "USD")},
		Contributions: []domain.GoalContribution{
			{Amount: money.New(50000, "USD"), MemberID: "a", At: time.Now()}, // $500 vs expected $200 → +$300 = 3 months ahead
		},
	}
	r := BuildPledgeReadout(g, "", 2)
	s := r.Standings[0]
	if s.AheadMonths != 3 {
		t.Fatalf("aheadMonths = %d, want 3", s.AheadMonths)
	}
	if s.Pace() != PledgePaceAhead {
		t.Fatalf("pace = %q, want ahead", s.Pace())
	}
}

func TestBuildPledgeReadoutBehind(t *testing.T) {
	g := domain.Goal{
		TargetAmount:  money.New(1000000, "USD"),
		Pledges:       map[string]money.Money{"a": money.New(10000, "USD")},
		Contributions: []domain.GoalContribution{{Amount: money.New(5000, "USD"), MemberID: "a", At: time.Now()}},
	}
	r := BuildPledgeReadout(g, "", 3) // expected 30000, actual 5000 → behind
	if r.Standings[0].Pace() != PledgePaceBehind {
		t.Fatalf("pace = %q, want behind", r.Standings[0].Pace())
	}
}

func TestBuildPledgeReadoutFallbackMember(t *testing.T) {
	g := domain.Goal{
		TargetAmount:  money.New(1000000, "USD"),
		Pledges:       map[string]money.Money{"me": money.New(10000, "USD")},
		Contributions: []domain.GoalContribution{{Amount: money.New(20000, "USD"), At: time.Now()}}, // no MemberID
	}
	r := BuildPledgeReadout(g, "me", 1)
	if r.Standings[0].Actual.Amount != 20000 {
		t.Fatalf("fallback attribution failed: actual=%d", r.Standings[0].Actual.Amount)
	}
}

func TestIsShared(t *testing.T) {
	if IsShared(domain.Goal{}) {
		t.Fatal("empty goal is not shared")
	}
	g := domain.Goal{Pledges: map[string]money.Money{"a": money.New(0, "USD")}}
	if IsShared(g) {
		t.Fatal("zero pledge is not shared")
	}
	g.Pledges["a"] = money.New(100, "USD")
	if !IsShared(g) {
		t.Fatal("positive pledge should be shared")
	}
}

func TestPledgeStartFrom(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// No contributions → created.
	if got := PledgeStartFrom(domain.Goal{}, created); !got.Equal(created) {
		t.Fatalf("want created, got %v", got)
	}
	early := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	g := domain.Goal{Contributions: []domain.GoalContribution{
		{At: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)},
		{At: early},
	}}
	if got := PledgeStartFrom(g, created); !got.Equal(early) {
		t.Fatalf("want earliest contribution %v, got %v", early, got)
	}
}
