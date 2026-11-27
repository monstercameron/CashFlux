// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestBulkReviewPreservesSplits(t *testing.T) {
	splitRow := domain.Transaction{
		ID: "s1", Desc: "Costco", Amount: money.New(-10000, "USD"),
		Tags: []string{reviewqueue.ReviewTag, "bulk"},
		Splits: []domain.CategorySplit{
			{CategoryID: "groceries", Amount: money.New(-6000, "USD")},
			{CategoryID: "household", Amount: money.New(-4000, "USD")},
		},
	}
	writes := Draft([]domain.Transaction{splitRow}, map[string]bool{"s1": true}, Edit{Kind: Review})
	if len(writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writes))
	}
	if !writes[0].Reviewed {
		t.Fatal("bulk review did not set Reviewed=true")
	}
	if len(writes[0].Splits) != 2 || !writes[0].SplitsReconcile() {
		t.Fatalf("bulk review destroyed splits: %+v", writes[0].Splits)
	}
	if HasTag(writes[0].Tags, reviewqueue.ReviewTag) {
		t.Fatalf("bulk review left review tag: %+v", writes[0].Tags)
	}
}

func TestCountNumberNegativeAndChooseCategoryAndTopFiveUncategorized(t *testing.T) {
	if got := CountNumber(-100); got != "-100" {
		t.Fatalf("CountNumber(-100) = %q, want -100", got)
	}
	if got := CountNumber(-100000); got != "-100,000" {
		t.Fatalf("CountNumber(-100000) = %q, want -100,000", got)
	}
	f := ChooseCategory(txnfilter.Criteria{Uncategorized: true}, "dining")
	if f.Uncategorized || f.Category != "dining" {
		t.Fatalf("ChooseCategory left contradictory state: %+v", f)
	}
	cats := []CategoryTotal{
		{ID: "c1", Minor: 600}, {ID: "c2", Minor: 500}, {ID: "c3", Minor: 400},
		{ID: "c4", Minor: 300}, {ID: "c5", Minor: 200}, {ID: "", Minor: 100}, {ID: "c6", Minor: 50},
	}
	p := Presentation{Text: func(k string, _ ...any) string { return k }}
	top := p.CategoryTopFive(cats, "__uncategorized__")
	foundUncat := false
	for _, c := range top {
		if c.ID == "" {
			foundUncat = true
		}
	}
	if !foundUncat {
		t.Fatalf("selected __uncategorized__ was folded into __other__: %+v", top)
	}
}
