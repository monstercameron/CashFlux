// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCreateBudgetWithCategorySavesBoth(t *testing.T) {
	a := newApp(t, false)
	cat := domain.Category{ID: "cat-pets", Name: "Pet care", Kind: domain.KindExpense}
	b := sharedBudget("bud-pets", "Pet care", cat.ID, 12000)

	if err := a.CreateBudgetWithCategory(&cat, b); err != nil {
		t.Fatalf("CreateBudgetWithCategory: %v", err)
	}

	var haveCat, haveBudget bool
	for _, c := range a.Categories() {
		if c.ID == cat.ID {
			haveCat = true
		}
	}
	for _, saved := range a.Budgets() {
		if saved.ID == b.ID && saved.CategoryID == cat.ID {
			haveBudget = true
		}
	}
	if !haveCat {
		t.Error("category was not saved")
	}
	if !haveBudget {
		t.Error("budget was not saved, or does not point at the new category")
	}
}

func TestCreateBudgetWithExistingCategory(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCategory(domain.Category{ID: "cat-food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("seed category: %v", err)
	}

	if err := a.CreateBudgetWithCategory(nil, sharedBudget("bud-food", "Food", "cat-food", 30000)); err != nil {
		t.Fatalf("CreateBudgetWithCategory: %v", err)
	}
	if got := len(a.Categories()); got != 1 {
		t.Errorf("categories = %d, want 1 (nil newCat must not mint one)", got)
	}
	if got := len(a.Budgets()); got != 1 {
		t.Errorf("budgets = %d, want 1", got)
	}
}

// A budget that fails validation must take its would-be category down with it:
// half a creation is what left orphan categories behind and made a re-submit
// silently reuse them (C584).
func TestCreateBudgetWithCategoryRollsBackTheCategory(t *testing.T) {
	a := newApp(t, false)
	cat := domain.Category{ID: "cat-pets", Name: "Pet care", Kind: domain.KindExpense}
	bad := sharedBudget("bud-pets", "Pet care", cat.ID, 0) // a non-positive limit is invalid
	bad.Limit = money.New(0, "USD")

	err := a.CreateBudgetWithCategory(&cat, bad)
	if err == nil {
		t.Fatal("CreateBudgetWithCategory returned nil for an invalid budget")
	}
	if !strings.Contains(err.Error(), "save budget") {
		t.Errorf("error = %q, want it to name the budget write that failed", err)
	}
	if got := len(a.Categories()); got != 0 {
		t.Errorf("categories = %d, want 0 — the category must be rolled back", got)
	}
	if got := len(a.Budgets()); got != 0 {
		t.Errorf("budgets = %d, want 0", got)
	}
}

// A category the store rejects must stop the whole operation before any budget
// exists — never a budget pointing at a category that was never written.
func TestCreateBudgetWithCategoryStopsOnCategoryFailure(t *testing.T) {
	a := newApp(t, false)
	bad := domain.Category{ID: "", Name: "Pet care", Kind: domain.KindExpense} // no ID: invalid

	err := a.CreateBudgetWithCategory(&bad, sharedBudget("bud-pets", "Pet care", "", 12000))
	if err == nil {
		t.Fatal("CreateBudgetWithCategory returned nil for an invalid category")
	}
	if !strings.Contains(err.Error(), "create category") {
		t.Errorf("error = %q, want it to name the category write that failed", err)
	}
	if got := len(a.Budgets()); got != 0 {
		t.Errorf("budgets = %d, want 0 — no budget may outlive a failed category write", got)
	}
}
