// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestComputeUnbudgeted(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	cats := []domain.Category{
		{ID: "food", Name: "Food", Kind: domain.KindExpense},
		{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense, ParentID: "food"},
		{ID: "dining", Name: "Dining", Kind: domain.KindExpense},
		{ID: "pets", Name: "Pets", Kind: domain.KindExpense},
	}
	// A budget tracks Food (so groceries, a child, is covered via descendants).
	budgets := []domain.Budget{
		{ID: "b1", Name: "Food", CategoryID: "food", Limit: money.New(50000, "USD")},
	}

	txns := []domain.Transaction{
		{ID: "1", Date: mid, CategoryID: "groceries", Amount: money.New(-20000, "USD")}, // covered (child of food)
		{ID: "2", Date: mid, CategoryID: "dining", Amount: money.New(-8000, "USD")},     // unbudgeted
		{ID: "3", Date: mid, CategoryID: "pets", Amount: money.New(-3000, "USD")},       // unbudgeted
		{ID: "4", Date: mid, CategoryID: "pets", Amount: money.New(-1000, "USD")},       // unbudgeted (adds)
		{ID: "5", Date: mid, Amount: money.New(-500, "USD")},                            // uncategorized
		{ID: "6", Date: before, CategoryID: "dining", Amount: money.New(-9000, "USD")},  // out of period
		{ID: "7", Date: mid, CategoryID: "salary", Amount: money.New(300000, "USD")},    // income, ignored
	}

	u, err := ComputeUnbudgeted(budgets, cats, txns, start, end, rates)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	// dining 8000 + pets 4000 + uncategorized 500 = 12500
	if u.Total.Amount != 12500 {
		t.Errorf("total = %d, want 12500", u.Total.Amount)
	}
	if len(u.Categories) != 3 {
		t.Fatalf("want 3 categories, got %d: %+v", len(u.Categories), u.Categories)
	}
	if u.Categories[0].CategoryID != "dining" || u.Categories[0].Amount.Amount != 8000 {
		t.Errorf("cat[0] = %+v", u.Categories[0])
	}
	if u.Categories[1].CategoryID != "pets" || u.Categories[1].Amount.Amount != 4000 {
		t.Errorf("cat[1] = %+v", u.Categories[1])
	}
	if u.Categories[2].CategoryID != "" || u.Categories[2].Amount.Amount != 500 {
		t.Errorf("cat[2] (uncategorized) = %+v", u.Categories[2])
	}
}

func TestComputeUnbudgetedSplitsAndNoBudgets(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	cats := []domain.Category{
		{ID: "dining", Name: "Dining", Kind: domain.KindExpense},
		{ID: "pets", Name: "Pets", Kind: domain.KindExpense},
	}
	// No budgets at all: everything is unbudgeted; a split lands per line.
	txns := []domain.Transaction{
		{
			ID: "1", Date: mid, Amount: money.New(-10000, "USD"), CategoryID: "dining",
			Splits: []domain.CategorySplit{
				{CategoryID: "dining", Amount: money.New(-7000, "USD")},
				{CategoryID: "pets", Amount: money.New(-3000, "USD")},
			},
		},
	}
	u, err := ComputeUnbudgeted(nil, cats, txns, start, end, rates)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if u.Total.Amount != 10000 {
		t.Errorf("total = %d, want 10000", u.Total.Amount)
	}
	got := map[string]int64{}
	for _, c := range u.Categories {
		got[c.CategoryID] = c.Amount.Amount
	}
	if got["dining"] != 7000 || got["pets"] != 3000 {
		t.Errorf("split breakdown wrong: %+v", got)
	}
}
