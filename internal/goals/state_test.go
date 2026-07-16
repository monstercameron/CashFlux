// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestReachedCountsEarmarks verifies the first-class completion rule: a financial
// goal is reached when committed savings PLUS earmarks cover the target.
func TestReachedCountsEarmarks(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	base := domain.Goal{ID: "g", Kind: domain.GoalKindFinancial, TargetAmount: usd(100000)}

	// Only $600 saved → not reached.
	g := base
	g.CurrentAmount = usd(60000)
	if Reached(g, nil, now) {
		t.Fatal("under-funded goal should not be reached")
	}
	// $600 saved + $400 earmarked = $1000 → reached (earmarks count).
	g.Allocations = []domain.GoalAllocation{{AccountID: "a", Amount: usd(40000)}}
	if !Reached(g, nil, now) {
		t.Fatal("saved + earmarked covering the target should be reached")
	}
	// Earmarks alone can reach it (nothing moved).
	g2 := base
	g2.Allocations = []domain.GoalAllocation{{AccountID: "a", Amount: usd(100000)}}
	if !Reached(g2, nil, now) {
		t.Fatal("fully earmarked goal should be reached even with $0 saved")
	}
}

func TestClassifyAndCounts(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	past := now.AddDate(0, -1, 0)
	future := now.AddDate(0, 2, 0)

	completed := domain.Goal{ID: "done", Kind: domain.GoalKindFinancial, TargetAmount: usd(1000), CurrentAmount: usd(1000)}
	missed := domain.Goal{ID: "miss", Kind: domain.GoalKindFinancial, TargetAmount: usd(1000), CurrentAmount: usd(300), TargetDate: past}
	current := domain.Goal{ID: "cur", Kind: domain.GoalKindFinancial, TargetAmount: usd(1000), CurrentAmount: usd(300), TargetDate: future}
	paused := domain.Goal{ID: "pause", Kind: domain.GoalKindFinancial, TargetAmount: usd(1000), CurrentAmount: usd(300), TargetDate: past, PausedUntil: future}
	fund := domain.Goal{ID: "sink", Kind: domain.GoalKindFinancial, IsSinkingFund: true, TargetAmount: usd(1000), CurrentAmount: usd(100), TargetDate: past}

	cases := []struct {
		g    domain.Goal
		want GoalState
	}{
		{completed, StateCompleted},
		{missed, StateMissed},
		{current, StateCurrent},
		{paused, StateCurrent}, // paused overdue is NOT missed (chosen state)
	}
	for _, tc := range cases {
		if got := Classify(tc.g, nil, now); got != tc.want {
			t.Errorf("Classify(%s) = %s, want %s", tc.g.ID, got, tc.want)
		}
	}

	all := []domain.Goal{completed, missed, current, paused, fund}
	c := CountByState(all, nil, now, false)
	// fund (sinking) is excluded; paused counts as current.
	if c.Completed != 1 || c.Missed != 1 || c.Current != 2 {
		t.Fatalf("counts = %+v, want {Current:2 Missed:1 Completed:1}", c)
	}
	if c.Total() != 4 {
		t.Fatalf("total = %d, want 4", c.Total())
	}
}
