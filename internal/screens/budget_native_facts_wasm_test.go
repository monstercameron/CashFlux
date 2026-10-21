// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetSavingsAdapterRetainsWideProjectionAndTypeLabel(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "savings", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", MonthlySavings: money.New(2, "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	goal := domain.Goal{ID: "goal", Name: "Goal", AccountID: account.ID, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(math.MaxInt64, "USD"), CurrentAmount: money.Zero("USD"), TargetDate: now.AddDate(0, 1, 0)}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	rows := computeSavingsAccounts(app, "", now, "USD", currency.Rates{Base: "USD"})
	if len(rows) != 1 || rows[0].Type != humanizeType("savings") || rows[0].RateMonths != math.MaxInt64/2+1 || rows[0].DeltaMonths != math.MaxInt64/2 || rows[0].SyncMinor != 2 {
		t.Fatalf("WASM savings adapter narrowed the projection or lost row labels: %+v", rows)
	}
}

func TestBudgetAgeOfMoneyCacheRefreshesAfterTransactionEdit(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	income := domain.Transaction{ID: "income", AccountID: "cash", Desc: "Pay", Amount: money.New(1000, "USD"), Date: start}
	expense := domain.Transaction{ID: "expense", AccountID: "cash", Desc: "Food", Amount: money.New(-100, "USD"), Date: start.AddDate(0, 0, 10)}
	for _, txn := range []domain.Transaction{income, expense} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	rates := currency.Rates{Base: "USD"}
	first := budgetAgeOfMoney(app, app.Transactions(), "USD", rates)
	if !first.Ready || first.Days != 10 {
		t.Fatalf("initial cached cash history = %+v", first)
	}
	expense.Date = start.AddDate(0, 0, 20)
	if err := app.PutTransaction(expense); err != nil {
		t.Fatal(err)
	}
	if got := budgetAgeOfMoney(app, app.Transactions(), "USD", rates); !got.Ready || got.Days != 20 {
		t.Fatalf("transaction edit retained stale FIFO age: %+v", got)
	}
}
