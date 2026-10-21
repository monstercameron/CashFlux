// SPDX-License-Identifier: MIT

package allocateview

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareEligibilityAndSharedFacts(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	locked := now.Add(time.Hour)
	open := domain.Account{ID: "open", Name: "Open", OwnerID: "m1", Class: domain.ClassAsset, ExpectedReturnAPR: 4}
	lockedAccount := domain.Account{ID: "locked", OwnerID: "m1", Class: domain.ClassAsset, LockUntil: locked}
	debt := domain.Account{ID: "debt", Name: "Card", OwnerID: "m1", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-50000, "USD")}.WithRateAPR(20)
	goal := domain.Goal{ID: "g", Name: "Trip", OwnerID: "m1", TargetAmount: money.New(10000, "USD"), CurrentAmount: money.New(2500, "USD")}
	got, err := Prepare(Input{Accounts: []domain.Account{open, lockedAccount, debt}, Transactions: []domain.Transaction{{AccountID: "debt", Amount: money.New(10000, "USD")}}, Goals: []domain.Goal{goal}, Now: now, ActiveMember: "m1", Mode: "fill", Weights: allocate.Weights{Returns: 1, GoalProgress: 1}, Amount: "100.00", Decimals: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 3 || got.Candidates[1].ID != "debt" {
		t.Fatalf("eligible candidates = %#v", got.Candidates)
	}
	if got.GoalFacts["g"].Progress != 25 || got.GoalFacts["g"].Remaining != 7500 {
		t.Fatalf("goal facts = %+v", got.GoalFacts["g"])
	}
	if got.Marginal[1].CapacityMinor != 40000 {
		t.Fatalf("debt capacity = %d", got.Marginal[1].CapacityMinor)
	}
}

func TestPrepareInvalidAmountsAreErrorsAndPreventSplit(t *testing.T) {
	got, err := Prepare(Input{Mode: "weighted", Amount: "nope", Reserve: "-1", MaxPer: "0.001", Decimals: 2, Weights: allocate.Weights{Returns: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalErr == nil || got.ReserveErr == nil || got.MaxPerErr == nil {
		t.Fatalf("parse errors = %v, %v, %v", got.TotalErr, got.ReserveErr, got.MaxPerErr)
	}
	if len(got.PlanByID) != 0 {
		t.Fatalf("invalid input produced plan: %#v", got.PlanByID)
	}
}

func TestPrepareMixedCurrencyGoalFactsUnavailable(t *testing.T) {
	g := domain.Goal{ID: "g", OwnerID: "m", TargetAmount: money.New(100, "EUR"), CurrentAmount: money.New(50, "USD")}
	got, err := Prepare(Input{Goals: []domain.Goal{g}, ActiveMember: "m", Mode: "weighted"})
	if err != nil {
		t.Fatal(err)
	}
	f := got.GoalFacts["g"]
	if f.ProgressAvailable || f.RemainingAvailable || f.Err == nil || f.Progress != 0 {
		t.Fatalf("mixed-currency goal facts = %+v", f)
	}
}

func TestPrepareSourceFailureDoesNotLookLikeEmptySnapshot(t *testing.T) {
	_, err := Prepare(Input{SourceErr: errors.New("accounts unavailable")})
	if err == nil {
		t.Fatal("source failure returned an empty successful view")
	}
}

func TestPrepareBalanceErrorIsRecorded(t *testing.T) {
	a := domain.Account{ID: "bad", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(1, "USD")}
	txn := domain.Transaction{AccountID: "bad", Amount: money.New(1, "EUR")}
	got, err := Prepare(Input{Accounts: []domain.Account{a}, Transactions: []domain.Transaction{txn}})
	if err != nil {
		t.Fatal(err)
	}
	if got.BalanceErrors["bad"] == nil {
		t.Fatalf("balance error missing: %+v", got.BalanceErrors)
	}
}

func TestPrepareDebtCapacityHandlesMinimumInt64(t *testing.T) {
	a := domain.Account{ID: "debt", Name: "Debt", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(math.MinInt64, "USD")}.WithRateAPR(1)
	got, err := Prepare(Input{Accounts: []domain.Account{a}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Marginal) != 1 || got.Marginal[0].CapacityMinor != math.MaxInt64 {
		t.Fatalf("debt capacity = %+v", got.Marginal)
	}
}
