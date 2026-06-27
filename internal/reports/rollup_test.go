// SPDX-License-Identifier: MIT

package reports

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestRollUpByParent(t *testing.T) {
	cats := []domain.Category{
		{ID: "food", Name: "Food"},
		{ID: "groceries", Name: "Groceries", ParentID: "food"},
		{ID: "dining", Name: "Dining", ParentID: "food"},
		{ID: "coffee", Name: "Coffee", ParentID: "dining"}, // nested two deep
		{ID: "rent", Name: "Rent"},
	}
	rows := []CategorySpend{
		{CategoryID: "groceries", Amount: 30000, Prior: 20000},
		{CategoryID: "dining", Amount: 10000},
		{CategoryID: "coffee", Amount: 5000},
		{CategoryID: "rent", Amount: 150000},
		{CategoryID: "", Amount: 1000}, // uncategorized stays itself
	}
	got := RollUpByParent(rows, cats)

	byID := map[string]CategorySpend{}
	for _, r := range got {
		byID[r.CategoryID] = r
	}
	// Food = groceries + dining + coffee (coffee rolls up through dining).
	if byID["food"].Amount != 45000 {
		t.Errorf("food rolled-up amount = %d, want 45000", byID["food"].Amount)
	}
	if !byID["food"].HasDelta || byID["food"].Prior != 20000 {
		t.Errorf("food prior=%d delta=%v, want prior 20000 + delta", byID["food"].Prior, byID["food"].HasDelta)
	}
	if byID["rent"].Amount != 150000 {
		t.Errorf("rent (no children) = %d, want 150000", byID["rent"].Amount)
	}
	if byID[""].Amount != 1000 {
		t.Errorf("uncategorized = %d, want 1000", byID[""].Amount)
	}
	// Children must not appear as separate rows.
	for _, id := range []string{"groceries", "dining", "coffee"} {
		if _, ok := byID[id]; ok {
			t.Errorf("child %q should have rolled up, but appears as its own row", id)
		}
	}
	// Sorted largest first.
	if got[0].CategoryID != "rent" {
		t.Errorf("first row = %q, want rent (largest)", got[0].CategoryID)
	}
}

// C238: when a comparison period ran but a rolled-up category had zero prior
// spend, RollUpByParent must set PriorZero=true so the UI can show "new".
func TestRollUpByParentPriorZero(t *testing.T) {
	cats := []domain.Category{
		{ID: "food", Name: "Food"},
		{ID: "tech", Name: "Tech"}, // new this period — no prior
	}
	// "food" has a non-zero prior (comparison ran); "tech" has zero prior.
	rows := []CategorySpend{
		{CategoryID: "food", Amount: 10000, Prior: 8000, HasDelta: true, DeltaPct: 25},
		{CategoryID: "tech", Amount: 5000, Prior: 0, PriorZero: true},
	}
	got := RollUpByParent(rows, cats)
	byID := map[string]CategorySpend{}
	for _, r := range got {
		byID[r.CategoryID] = r
	}
	if byID["food"].PriorZero {
		t.Error("food (non-zero prior) must not have PriorZero set after rollup")
	}
	if !byID["tech"].PriorZero {
		t.Error("tech (zero prior, comparison ran) must have PriorZero set after rollup")
	}
}
