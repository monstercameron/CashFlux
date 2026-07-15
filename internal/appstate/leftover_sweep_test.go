// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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
