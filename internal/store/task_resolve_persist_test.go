// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestTaskResolveRoundTrip proves a task's self-resolve rule (XC8) survives an
// export/import round-trip losslessly — both the formula condition and the
// structured matcher fields.
func TestTaskResolveRoundTrip(t *testing.T) {
	s := newStore(t)
	task := domain.Task{
		ID: "task-1", Title: "Chase the duplicate charge", Status: domain.StatusOpen,
		Priority: domain.PriorityMedium,
		Resolve: &domain.TaskResolve{
			Condition: "x > 0", MatchPayee: "Acme", MatchAmountMinor: 4200,
			MatchCurrency: "USD", MatchToleranceMinor: 100, MatchRefund: true,
		},
	}
	if err := s.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, _ := s2.ListTasks()
	if len(got) != 1 || got[0].Resolve == nil {
		t.Fatalf("resolve rule lost on round-trip: %+v", got)
	}
	r := got[0].Resolve
	if r.Condition != "x > 0" || r.MatchPayee != "Acme" || r.MatchAmountMinor != 4200 ||
		r.MatchCurrency != "USD" || r.MatchToleranceMinor != 100 || !r.MatchRefund {
		t.Fatalf("resolve fields mismatch: %+v", r)
	}
}
