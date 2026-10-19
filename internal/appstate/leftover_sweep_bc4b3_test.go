// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestApplyLeftoverSweepConvertsBeforeMergingAllocation(t *testing.T) {
	app := newAllocApp(t)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 0.8}}); err != nil {
		t.Fatal(err)
	}
	account := allocSeedAccount(t, app, "bc4b3-checking", "Checking", domain.TypeChecking)
	sweepDeposit(t, app, account.ID, 500000)
	goal := allocSeedGoal(t, app, "bc4b3-goal", "Emergency fund", 1000000, 0)
	goal.AccountID = account.ID
	goal.Allocations = []domain.GoalAllocation{{AccountID: account.ID, Amount: money.New(20000, "EUR")}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}

	got, err := app.ApplyLeftoverSweep(goal.ID, 15000, "USD") // $150 = €187.50
	if err != nil {
		t.Fatalf("ApplyLeftoverSweep: %v", err)
	}
	if len(got.Allocations) != 1 || !got.Allocations[0].Amount.Equal(money.New(38750, "EUR")) {
		t.Fatalf("allocation = %+v, want €387.50 in EUR minor units", got.Allocations)
	}
}

func TestMergeSweepAmountRejectsOverflow(t *testing.T) {
	_, err := mergeSweepAmount(money.New(math.MaxInt64, "USD"), 1, "USD", currency.Rates{Base: "USD"})
	if err == nil {
		t.Fatal("mergeSweepAmount returned no error for overflowing allocation")
	}
}
