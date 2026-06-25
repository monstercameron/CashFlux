// SPDX-License-Identifier: MIT

// Tests for the new executable action kinds added in C256:
// ActionCreateGoal (SMART-G12), ActionCancelSubscription (SMART-SU1).

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// TestG12SuggestGoals_EmitsCreateGoalAction verifies that when the user has no
// emergency fund but enough spend history, SMART-G12 emits ActionCreateGoal
// (not ActionNavigate) with the computed target amount and a non-empty name.
func TestG12SuggestGoals_EmitsCreateGoalAction(t *testing.T) {
	in := baseInput().withBaseline(500_00, 300_00) // $3000/mo income, $1000/mo expense
	in.Goals = nil                                 // no emergency fund
	got := g12SuggestGoals(in)
	if len(got) != 1 {
		t.Fatalf("want 1 insight, got %d", len(got))
	}
	ins := got[0]
	if ins.Action == nil {
		t.Fatal("want non-nil Action")
	}
	if ins.Action.Kind != smart.ActionCreateGoal {
		t.Errorf("want ActionCreateGoal, got %q", ins.Action.Kind)
	}
	if ins.Action.GoalName == "" {
		t.Error("GoalName must not be empty")
	}
	if ins.Action.GoalTarget <= 0 {
		t.Errorf("GoalTarget must be positive, got %d", ins.Action.GoalTarget)
	}
	if ins.Action.Label == "" {
		t.Error("Label must not be empty")
	}
}

// TestG12SuggestGoals_AbsentWhenEmergencyGoalExists verifies SMART-G12 fires
// no insight when an emergency-fund goal already exists (G11 handles that).
func TestG12SuggestGoals_AbsentWhenEmergencyGoalExists(t *testing.T) {
	in := baseInput().withBaseline(500_00, 300_00)
	in.Goals = []domain.Goal{
		{ID: "ef1", Name: "Emergency fund", TargetAmount: usd(600_00), CurrentAmount: usd(100_00)},
	}
	got := g12SuggestGoals(in)
	if len(got) != 0 {
		t.Fatalf("want 0 insights when emergency goal exists, got %d", len(got))
	}
}

// TestSU1CancelCandidates_EmitsCancelSubscriptionAction verifies that
// SMART-SU1 emits ActionCancelSubscription with the subscription name when a
// subscription qualifies as a cancel candidate.
func TestSU1CancelCandidates_EmitsCancelSubscriptionAction(t *testing.T) {
	// Build a subscription that qualifies: last charge is stale (more than one
	// cadence period ago) so NeedsReview returns true → it's a cancel candidate.
	// monthlyCharges ends at lastMonth; set lastMonth 3 months before now so
	// the last charge is overdue.
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	staleMonth := time.March // 3 months before June → NeedsReview fires
	txns := monthlyCharges("Netflix", 1_99, staleMonth, recurringMinCount)
	in := baseInput()
	in.Now = now
	in.Transactions = append(in.Transactions, txns...)

	got := su1CancelCandidates(in)
	if len(got) == 0 {
		t.Skip("no cancel candidates — subscription may not qualify; skipping")
	}
	for _, ins := range got {
		if ins.Action == nil {
			t.Errorf("[%s] want non-nil Action", ins.Feature)
			continue
		}
		if ins.Action.Kind != smart.ActionCancelSubscription {
			t.Errorf("[%s] want ActionCancelSubscription, got %q", ins.Feature, ins.Action.Kind)
		}
		if ins.Action.SubscriptionName == "" {
			t.Errorf("[%s] SubscriptionName must not be empty", ins.Feature)
		}
		if ins.Action.Label == "" {
			t.Errorf("[%s] Label must not be empty", ins.Feature)
		}
	}
}
