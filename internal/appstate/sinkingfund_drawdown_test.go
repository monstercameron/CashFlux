// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestSinkingFundDrawdownOnExpense verifies C192: recording an expense in a
// category linked to a sinking fund draws the fund's balance down by the spend,
// income/transfers don't draw, and an edit (re-put) doesn't double-draw.
func TestSinkingFundDrawdownOnExpense(t *testing.T) {
	a, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.PutAccount(domain.Account{ID: "a1", Name: "Checking", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD"}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutCategory(domain.Category{ID: "carrepair", Name: "Car repairs", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	// A sinking fund with $300 saved, linked to the "carrepair" category.
	fund := domain.Goal{
		ID: "g1", Name: "Car repairs fund", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(50000, "USD"), CurrentAmount: money.New(30000, "USD"),
		IsSinkingFund: true, CategoryID: "carrepair",
	}
	if err := a.PutGoal(fund); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	balance := func() int64 {
		for _, g := range a.Goals() {
			if g.ID == "g1" {
				return g.CurrentAmount.Amount
			}
		}
		t.Fatal("fund g1 disappeared")
		return 0
	}

	// Income in the linked category must NOT draw the fund down.
	if err := a.PutTransaction(domain.Transaction{ID: "inc", AccountID: "a1", CategoryID: "carrepair", Desc: "refund", Amount: money.New(5000, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction income: %v", err)
	}
	if got := balance(); got != 30000 {
		t.Fatalf("income should not draw fund: balance = %d, want 30000", got)
	}

	// An expense in the linked category draws the fund down by the spend.
	if err := a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", CategoryID: "carrepair", Desc: "Brakes", Amount: money.New(-12000, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction expense: %v", err)
	}
	if got := balance(); got != 18000 {
		t.Fatalf("expense should draw fund to 18000, got %d", got)
	}

	// Re-putting the SAME transaction (an edit) must not draw again.
	if err := a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", CategoryID: "carrepair", Desc: "Brakes (edited)", Amount: money.New(-12000, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction edit: %v", err)
	}
	if got := balance(); got != 18000 {
		t.Fatalf("edit must not double-draw: balance = %d, want 18000", got)
	}

	// An expense in an UNLINKED category must not touch the fund.
	if err := a.PutCategory(domain.Category{ID: "dining", Name: "Dining", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory dining: %v", err)
	}
	if err := a.PutTransaction(domain.Transaction{ID: "t2", AccountID: "a1", CategoryID: "dining", Desc: "Lunch", Amount: money.New(-2000, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction dining: %v", err)
	}
	if got := balance(); got != 18000 {
		t.Fatalf("unlinked expense must not draw fund: balance = %d, want 18000", got)
	}

	// A spend larger than the balance floors the fund at zero (never negative).
	if err := a.PutTransaction(domain.Transaction{ID: "t3", AccountID: "a1", CategoryID: "carrepair", Desc: "Engine", Amount: money.New(-99999, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction big: %v", err)
	}
	if got := balance(); got != 0 {
		t.Fatalf("over-spend should floor fund at 0, got %d", got)
	}
}
