// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

// sweepDeposit posts a deposit so an account has a real balance to test against.
func sweepDeposit(t *testing.T, app *App, acctID string, amtMinor int64) {
	t.Helper()
	txn := domain.Transaction{
		ID:        "dep-" + acctID,
		AccountID: acctID,
		Date:      time.Now(),
		Payee:     "Opening",
		Desc:      "Opening deposit",
		Amount:    money.New(amtMinor, "USD"),
		Source:    domain.TxnSourceManual,
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
}

func TestApplyLeftoverSweepEarmarksGoal(t *testing.T) {
	app := newAllocApp(t)
	acc := allocSeedAccount(t, app, "checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, acc.ID, 500000) // $5,000 real balance
	g := allocSeedGoal(t, app, "emg", "Emergency fund", 1000000, 0)
	g.AccountID = acc.ID
	if err := app.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	if !app.SweepAllowedForGoal("emg") {
		t.Fatal("sweep should be allowed for a healthy goal")
	}

	got, err := app.ApplyLeftoverSweep("emg", 8700, "USD") // sweep $87
	if err != nil {
		t.Fatalf("ApplyLeftoverSweep: %v", err)
	}
	if got.AllocatedMinor() != 8700 {
		t.Errorf("earmarked = %d, want 8700", got.AllocatedMinor())
	}

	// A second sweep merges into the same account allocation.
	got, err = app.ApplyLeftoverSweep("emg", 1300, "USD")
	if err != nil {
		t.Fatalf("second ApplyLeftoverSweep: %v", err)
	}
	if got.AllocatedMinor() != 10000 {
		t.Errorf("merged earmark = %d, want 10000", got.AllocatedMinor())
	}
	if len(got.Allocations) != 1 {
		t.Errorf("allocations = %d, want 1 (merged)", len(got.Allocations))
	}
}

func TestApplyLeftoverSweepGatedByBreach(t *testing.T) {
	app := newAllocApp(t)
	acc := allocSeedAccount(t, app, "checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, acc.ID, 100000) // $1,000 real balance
	g := allocSeedGoal(t, app, "emg", "Emergency fund", 1000000, 0)
	g.AccountID = acc.ID
	// Already over-earmarked: $2,000 reserved against a $1,000 balance.
	g.Allocations = []domain.GoalAllocation{{AccountID: acc.ID, Amount: money.New(200000, "USD")}}
	if err := app.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	if app.SweepAllowedForGoal("emg") {
		t.Error("sweep should be blocked when the linked account is over-earmarked")
	}
	if _, err := app.ApplyLeftoverSweep("emg", 8700, "USD"); err == nil {
		t.Error("ApplyLeftoverSweep should refuse an over-earmarked goal")
	}
}

func TestApplyLeftoverSweepNoLinkedAccount(t *testing.T) {
	app := newAllocApp(t)
	allocSeedGoal(t, app, "emg", "Emergency fund", 1000000, 0)
	if _, err := app.ApplyLeftoverSweep("emg", 8700, "USD"); err == nil {
		t.Error("sweep into a goal with no linked account should error")
	}
}

func TestApplyLeftoverSweepRefusesPostSweepBreach(t *testing.T) {
	app := newAllocApp(t)
	acc := allocSeedAccount(t, app, "capacity-checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, acc.ID, 100000) // $1,000 balance
	goal := allocSeedGoal(t, app, "capacity-goal", "Emergency fund", 1000000, 0)
	goal.AccountID = acc.ID
	goal.Allocations = []domain.GoalAllocation{{AccountID: acc.ID, Amount: money.New(90000, "USD")}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	capacity, err := app.LeftoverSweepCapacity(goal.ID, "USD")
	if err != nil || capacity != 10000 {
		t.Fatalf("sweep capacity = %d, %v; want 10000", capacity, err)
	}
	if _, err := app.ApplyLeftoverSweep(goal.ID, 20000, "USD"); err == nil {
		t.Fatal("sweep that would earmark $1,100 against a $1,000 account must be refused")
	}
	got := app.Goals()[0]
	if got.AllocatedMinor() != 90000 {
		t.Fatalf("allocation changed after refused sweep: %d, want 90000", got.AllocatedMinor())
	}
}

func TestApplyLeftoverSweepNewAllocationUsesAccountCurrency(t *testing.T) {
	app := newAllocApp(t)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 0.8}}); err != nil {
		t.Fatal(err)
	}
	acc := allocSeedAccount(t, app, "currency-checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, acc.ID, 500000)
	goal := allocSeedGoal(t, app, "currency-goal", "Emergency fund", 1000000, 0)
	goal.AccountID = acc.ID
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	got, err := app.ApplyLeftoverSweep(goal.ID, 8000, "EUR") // €80 = $64 at this rate
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Allocations) != 1 || !got.Allocations[0].Amount.Equal(money.New(6400, "USD")) {
		t.Fatalf("allocation = %+v, want $64.00 in account currency", got.Allocations)
	}
}

func TestApplyLeftoverSweepDoesNotMutateCachedGoalOnRefusedWrite(t *testing.T) {
	app := newAllocApp(t)
	acc := allocSeedAccount(t, app, "refused-checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, acc.ID, 500000)
	goal := allocSeedGoal(t, app, "refused-goal", "Emergency fund", 1000000, 0)
	goal.AccountID = acc.ID
	goal.Allocations = []domain.GoalAllocation{{AccountID: acc.ID, Amount: money.New(10000, "USD")}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	app.activeRoleFn = func() domain.MemberRole { return domain.RoleViewer }
	for i := 0; i < 2; i++ {
		if _, err := app.ApplyLeftoverSweep(goal.ID, 5000, "USD"); err == nil {
			t.Fatal("viewer write unexpectedly succeeded")
		}
	}
	app.activeRoleFn = nil
	got := app.Goals()[0]
	if got.AllocatedMinor() != 10000 {
		t.Fatalf("cached allocation after refused writes = %d, want 10000", got.AllocatedMinor())
	}
	got, err := app.ApplyLeftoverSweep(goal.ID, 5000, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if got.AllocatedMinor() != 15000 {
		t.Fatalf("allocation after successful write = %d, want 15000", got.AllocatedMinor())
	}
}
