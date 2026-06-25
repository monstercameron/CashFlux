// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func dt(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

// expense builds a non-transfer negative (spend) transaction in USD.
func expense(cat string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{CategoryID: cat, Amount: money.New(-major*100, "USD"), Date: on}
}

func usdRates() currency.Rates { return currency.Rates{Base: "USD"} }

func TestSpendingByCategorySortedAndExcludes(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		expense("food", 100, dt(2026, time.June, 5)),
		expense("food", 50, dt(2026, time.June, 20)),
		expense("rent", 900, dt(2026, time.June, 1)),
		expense("food", 999, dt(2026, time.May, 31)),                                                              // out of range — excluded
		{CategoryID: "x", Amount: money.New(5000, "USD"), Date: dt(2026, time.June, 10)},                          // income — excluded
		{CategoryID: "y", Amount: money.New(-7000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 10)}, // transfer — excluded
	}
	got, err := SpendingByCategory(txns, start, end, false, time.Time{}, time.Time{}, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d categories, want 2 (food, rent): %+v", len(got), got)
	}
	// rent (900) sorts before food (150); largest first.
	if got[0].CategoryID != "rent" || got[0].Amount != 90000 {
		t.Errorf("row 0 = %+v, want rent 90000", got[0])
	}
	if got[1].CategoryID != "food" || got[1].Amount != 15000 {
		t.Errorf("row 1 = %+v, want food 15000", got[1])
	}
	if got[0].HasDelta {
		t.Error("HasDelta should be false without a comparison")
	}
	if Total(got) != 105000 {
		t.Errorf("Total = %d, want 105000", Total(got))
	}
}

// C58: a split transaction attributes each line to its own category, not the
// whole-transaction category — and a receipt-imported split with an empty
// transaction category is no longer invisible.
func TestSpendingByCategorySplits(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	// One $100 grocery charge, split $60 produce / $40 household. The
	// transaction's own CategoryID is empty (the receipt-import case).
	split := domain.Transaction{
		CategoryID: "", Amount: money.New(-10000, "USD"), Date: dt(2026, time.June, 5),
		Splits: []domain.CategorySplit{
			{CategoryID: "produce", Amount: money.New(-6000, "USD")},
			{CategoryID: "household", Amount: money.New(-4000, "USD")},
		},
	}
	got, err := SpendingByCategory([]domain.Transaction{split, expense("rent", 900, dt(2026, time.June, 1))}, start, end, false, time.Time{}, time.Time{}, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byCat := map[string]int64{}
	for _, c := range got {
		byCat[c.CategoryID] = c.Amount
	}
	if byCat["produce"] != 6000 {
		t.Errorf("produce = %d, want 6000", byCat["produce"])
	}
	if byCat["household"] != 4000 {
		t.Errorf("household = %d, want 4000", byCat["household"])
	}
	if _, ok := byCat[""]; ok {
		t.Errorf("empty whole-transaction category must not appear: %+v", got)
	}
	// Total is unchanged: $100 split + $900 rent = $1000, no double count.
	if Total(got) != 100000 {
		t.Errorf("Total = %d, want 100000 (no double count)", Total(got))
	}
}

func TestSpendingByCategoryComparison(t *testing.T) {
	curStart, curEnd := dt(2026, time.June, 1), dt(2026, time.July, 1)
	priStart, priEnd := dt(2026, time.May, 1), dt(2026, time.June, 1)
	txns := []domain.Transaction{
		// food: $100 prior, $150 current → +50%
		expense("food", 100, dt(2026, time.May, 10)),
		expense("food", 150, dt(2026, time.June, 10)),
		// fun: $200 prior, $0 current → -100%, still listed as a mover
		expense("fun", 200, dt(2026, time.May, 15)),
		// new: $0 prior, $80 current → no baseline (HasDelta false)
		expense("new", 80, dt(2026, time.June, 20)),
	}
	got, err := SpendingByCategory(txns, curStart, curEnd, true, priStart, priEnd, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	by := map[string]CategorySpend{}
	for _, r := range got {
		by[r.CategoryID] = r
	}
	if len(got) != 3 {
		t.Fatalf("got %d categories, want 3 (food, fun, new): %+v", len(got), got)
	}
	if f := by["food"]; f.Amount != 15000 || f.Prior != 10000 || !f.HasDelta || f.DeltaPct != 50 {
		t.Errorf("food = %+v, want amount 15000 prior 10000 delta +50", f)
	}
	if fn := by["fun"]; fn.Amount != 0 || fn.Prior != 20000 || !fn.HasDelta || fn.DeltaPct != -100 {
		t.Errorf("fun = %+v, want amount 0 prior 20000 delta -100", fn)
	}
	if n := by["new"]; n.Amount != 8000 || n.Prior != 0 || n.HasDelta {
		t.Errorf("new = %+v, want amount 8000 prior 0 no-delta", n)
	}
}
