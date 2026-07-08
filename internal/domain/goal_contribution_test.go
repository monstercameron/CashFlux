// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func contrib(minor int64, txn string) GoalContribution {
	return GoalContribution{Amount: money.New(minor, "USD"), TxnID: txn, At: time.Unix(minor, 0)}
}

func TestRecordAndPopContribution(t *testing.T) {
	var g Goal
	g = g.RecordContribution(contrib(100, ""))
	g = g.RecordContribution(contrib(200, "tx2"))
	if len(g.Contributions) != 2 {
		t.Fatalf("len = %d, want 2", len(g.Contributions))
	}

	g2, last, ok := g.PopLastContribution()
	if !ok {
		t.Fatal("expected a contribution to pop")
	}
	if last.Amount.Amount != 200 || last.TxnID != "tx2" {
		t.Errorf("popped %+v, want amount 200 / txn tx2", last)
	}
	if len(g2.Contributions) != 1 {
		t.Errorf("after pop len = %d, want 1", len(g2.Contributions))
	}
	// Original is not mutated (value semantics / copy-on-write).
	if len(g.Contributions) != 2 {
		t.Errorf("original mutated: len = %d, want 2", len(g.Contributions))
	}

	// Popping down to empty then once more is a safe no-op.
	g3, _, _ := g2.PopLastContribution()
	if _, _, ok := g3.PopLastContribution(); ok {
		t.Error("popping an empty log should return ok=false")
	}
}

func TestRecordContributionCap(t *testing.T) {
	var g Goal
	for i := 0; i < MaxGoalContributions+10; i++ {
		g = g.RecordContribution(contrib(int64(i), ""))
	}
	if len(g.Contributions) != MaxGoalContributions {
		t.Fatalf("len = %d, want cap %d", len(g.Contributions), MaxGoalContributions)
	}
	// The oldest 10 were dropped; the newest is the last recorded.
	if got := g.Contributions[len(g.Contributions)-1].Amount.Amount; got != int64(MaxGoalContributions+9) {
		t.Errorf("newest = %d, want %d", got, MaxGoalContributions+9)
	}
	if got := g.Contributions[0].Amount.Amount; got != 10 {
		t.Errorf("oldest kept = %d, want 10", got)
	}
}
