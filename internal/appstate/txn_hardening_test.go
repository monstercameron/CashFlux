// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPutTransactionMinInt64DoesNotPanicSinkingFundDrawdown(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutGoal(domain.Goal{
		ID: "sf", Name: "Car Repair", OwnerID: "m1", Scope: domain.ScopeIndividual,
		CategoryID: "cat-auto", IsSinkingFund: true,
		TargetAmount: money.New(50000, "USD"), CurrentAmount: money.New(20000, "USD"),
	}); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	tx := domain.Transaction{
		ID: "min-tx", AccountID: "chk", CategoryID: "cat-auto",
		Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Desc: "Extreme", Amount: money.New(math.MinInt64, "USD"),
	}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction(MinInt64): %v", err)
	}
}
