// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func sharedBudget(idStr, name, catID string, limit int64) domain.Budget {
	return domain.Budget{
		ID: idStr, Name: name, CategoryID: catID, Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(limit, "USD"),
	}
}

func TestCoverBudget(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutBudget(sharedBudget("shop", "Shopping", "c-shop", 40000)); err != nil {
		t.Fatalf("seed shop: %v", err)
	}
	if err := a.PutBudget(sharedBudget("groc", "Groceries", "c-groc", 60000)); err != nil {
		t.Fatalf("seed groc: %v", err)
	}

	if err := a.CoverBudget("shop", "groc", money.New(30400, "USD")); err != nil {
		t.Fatalf("CoverBudget: %v", err)
	}

	limits := map[string]int64{}
	for _, b := range a.Budgets() {
		limits[b.ID] = b.Limit.Amount
	}
	if limits["shop"] != 9600 {
		t.Errorf("source limit = %d, want 9600 (persisted)", limits["shop"])
	}
	if limits["groc"] != 90400 {
		t.Errorf("destination limit = %d, want 90400 (persisted)", limits["groc"])
	}
	if total := limits["shop"] + limits["groc"]; total != 100000 {
		t.Errorf("total budgeted changed to %d, want 100000 (balanced)", total)
	}
}

func TestCoverBudgetRejectsDrainingSource(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutBudget(sharedBudget("shop", "Shopping", "c-shop", 40000)); err != nil {
		t.Fatalf("seed shop: %v", err)
	}
	if err := a.PutBudget(sharedBudget("groc", "Groceries", "c-groc", 60000)); err != nil {
		t.Fatalf("seed groc: %v", err)
	}

	// Moving the source's entire limit would leave it at zero — invalid for a budget.
	err := a.CoverBudget("shop", "groc", money.New(40000, "USD"))
	if !errors.Is(err, budgeting.ErrInsufficientSource) {
		t.Fatalf("err = %v, want ErrInsufficientSource", err)
	}
	// Nothing was persisted.
	for _, b := range a.Budgets() {
		want := int64(40000)
		if b.ID == "groc" {
			want = 60000
		}
		if b.Limit.Amount != want {
			t.Errorf("%s limit = %d, want %d (unchanged)", b.ID, b.Limit.Amount, want)
		}
	}
}

func TestCoverBudgetMissingBudget(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutBudget(sharedBudget("shop", "Shopping", "c-shop", 40000)); err != nil {
		t.Fatalf("seed shop: %v", err)
	}
	if err := a.CoverBudget("shop", "nope", money.New(100, "USD")); err == nil {
		t.Error("expected error for a missing destination budget")
	}
}
