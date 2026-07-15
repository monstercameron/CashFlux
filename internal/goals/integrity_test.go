// SPDX-License-Identifier: MIT

package goals

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func earmarkGoal(id, acct string, amtMinor int64) domain.Goal {
	return domain.Goal{
		ID:          id,
		Allocations: []domain.GoalAllocation{{AccountID: acct, Amount: money.New(amtMinor, "USD")}},
	}
}

func TestEarmarkIntegrity(t *testing.T) {
	goals := []domain.Goal{
		earmarkGoal("emg", "checking", 200000),
		earmarkGoal("vac", "checking", 50000),
		earmarkGoal("car", "savings", 30000),
	}
	balances := map[string]int64{
		"checking": 140000, // 250000 earmarked > 140000 -> breach of 110000
		"savings":  100000, // 30000 earmarked < 100000 -> fine
	}
	got := EarmarkIntegrity(goals, balances)
	if len(got) != 2 {
		t.Fatalf("accounts = %d, want 2", len(got))
	}
	// sorted by id: checking, savings
	if got[0].AccountID != "checking" {
		t.Fatalf("first account = %q, want checking", got[0].AccountID)
	}
	if got[0].EarmarkedMinor != 250000 {
		t.Errorf("checking earmarked = %d, want 250000", got[0].EarmarkedMinor)
	}
	if !got[0].Breached() {
		t.Error("checking should be breached")
	}
	if got[0].OverMinor() != 110000 {
		t.Errorf("checking over = %d, want 110000", got[0].OverMinor())
	}
	if got[1].Breached() {
		t.Error("savings should not be breached")
	}
	if got[1].OverMinor() != 0 {
		t.Errorf("savings over = %d, want 0", got[1].OverMinor())
	}
}

func TestEarmarkIntegrityUnknownAccountIsBreach(t *testing.T) {
	goals := []domain.Goal{earmarkGoal("emg", "ghost", 100)}
	got := EarmarkIntegrity(goals, map[string]int64{})
	if len(got) != 1 || !got[0].Breached() {
		t.Fatalf("earmark against unknown account should breach: %+v", got)
	}
}

func TestGoalSweepAllowed(t *testing.T) {
	// emg linked to checking (breached); car linked to savings (fine).
	emg := domain.Goal{ID: "emg", AccountID: "checking"}
	car := domain.Goal{ID: "car", AccountID: "savings"}
	allGoals := []domain.Goal{
		earmarkGoal("emg", "checking", 200000),
		earmarkGoal("car", "savings", 30000),
	}
	balances := map[string]int64{"checking": 140000, "savings": 100000}

	if GoalSweepAllowed(emg, allGoals, balances) {
		t.Error("sweep into goal on breached account should be disallowed")
	}
	if !GoalSweepAllowed(car, allGoals, balances) {
		t.Error("sweep into goal on healthy account should be allowed")
	}
	// A goal with no linked account is always allowed.
	free := domain.Goal{ID: "free"}
	if !GoalSweepAllowed(free, allGoals, balances) {
		t.Error("goal with no account should be allowed")
	}
}

func TestAccountBreached(t *testing.T) {
	goals := []domain.Goal{earmarkGoal("emg", "checking", 200000)}
	if !AccountBreached(goals, "checking", 140000) {
		t.Error("checking should be breached")
	}
	if AccountBreached(goals, "checking", 250000) {
		t.Error("checking with ample balance should not be breached")
	}
}
