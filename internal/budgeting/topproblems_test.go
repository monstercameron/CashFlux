// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// mkStatus builds a Status for the table tests without running a full evaluation:
// spent/remaining are minor USD units (a negative remaining = overspent).
func mkStatus(id string, state State, spent, remaining int64, pct int) Status {
	return Status{
		Budget:    domain.Budget{ID: id, CategoryID: id},
		Spent:     money.New(spent, "USD"),
		Remaining: money.New(remaining, "USD"),
		Percent:   pct,
		State:     state,
	}
}

func ids(ps []Problem) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Status.Budget.ID
	}
	return out
}

func TestTopProblems(t *testing.T) {
	over1 := mkStatus("over1", StateOver, 12000, -2000, 120) // $20 over
	over2 := mkStatus("over2", StateOver, 15000, -5000, 150) // $50 over (deeper)
	near1 := mkStatus("near1", StateNear, 9000, 1000, 90)
	near2 := mkStatus("near2", StateNear, 8500, 1500, 85)
	ok1 := mkStatus("ok1", StateOK, 3000, 7000, 30)
	pace1 := mkStatus("pace1", StateOK, 6000, 4000, 60) // healthy state, but pace-flagged

	tests := []struct {
		name     string
		statuses []Status
		pace     map[string]bool
		n        int
		want     []string
	}{
		{
			name:     "over budgets lead, deepest overspend first",
			statuses: []Status{over1, over2, near1, ok1},
			n:        3,
			want:     []string{"over2", "over1", "near1"},
		},
		{
			name:     "near ranks below over, by percent used",
			statuses: []Status{near2, near1, over1},
			n:        3,
			want:     []string{"over1", "near1", "near2"},
		},
		{
			name:     "healthy budgets are excluded",
			statuses: []Status{ok1, near1},
			n:        5,
			want:     []string{"near1"},
		},
		{
			name:     "pace risk surfaces a healthy-state budget below near",
			statuses: []Status{pace1, near1},
			pace:     map[string]bool{"pace1": true},
			n:        3,
			want:     []string{"near1", "pace1"},
		},
		{
			name:     "n caps the result",
			statuses: []Status{over1, over2, near1},
			n:        1,
			want:     []string{"over2"},
		},
		{
			name:     "n<=0 returns nil",
			statuses: []Status{over1},
			n:        0,
			want:     nil,
		},
		{
			name:     "nil pace map is safe",
			statuses: []Status{ok1},
			n:        3,
			want:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(TopProblems(tc.statuses, tc.pace, tc.n))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestTopProblemsPaceOnlyFlag verifies the PaceRisk field is set only when a
// budget is surfaced by its pace, not by an over/near state that is also
// coincidentally pace-flagged.
func TestTopProblemsPaceOnlyFlag(t *testing.T) {
	over := mkStatus("over", StateOver, 12000, -2000, 120)
	pace := mkStatus("pace", StateOK, 6000, 4000, 60)
	got := TopProblems([]Status{over, pace}, map[string]bool{"over": true, "pace": true}, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 problems, got %d", len(got))
	}
	byID := map[string]Problem{}
	for _, p := range got {
		byID[p.Status.Budget.ID] = p
	}
	if byID["over"].PaceRisk {
		t.Errorf("over-budget item should not be marked PaceRisk")
	}
	if !byID["pace"].PaceRisk {
		t.Errorf("pace-only item should be marked PaceRisk")
	}
}
