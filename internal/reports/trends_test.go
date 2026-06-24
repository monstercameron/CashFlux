// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCategoryTrendsSeriesSortAndDelta(t *testing.T) {
	// Three monthly buckets: April, May, June 2026.
	bounds := []time.Time{
		dt(2026, time.April, 1), dt(2026, time.May, 1),
		dt(2026, time.June, 1), dt(2026, time.July, 1),
	}
	txns := []domain.Transaction{
		expense("rent", 500, dt(2026, time.April, 10)),
		expense("rent", 500, dt(2026, time.May, 10)),
		expense("rent", 1000, dt(2026, time.June, 10)),
		expense("food", 100, dt(2026, time.April, 12)),
		expense("food", 200, dt(2026, time.May, 12)),
		expense("food", 50, dt(2026, time.June, 12)),
		expense("once", 300, dt(2026, time.May, 20)),
		incomeTxn("salary", 4000, dt(2026, time.May, 1)),                                       // income — excluded
		expense("food", 999, dt(2026, time.March, 31)),                                         // out of range — excluded
		{Amount: money.New(-1000, "USD"), TransferAccountID: "a", Date: dt(2026, time.May, 9)}, // transfer — excluded
	}

	got, err := CategoryTrends(txns, bounds, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d categories, want 3 (rent, food, once): %+v", len(got), got)
	}

	// Sorted by Total descending: rent (200000) > food (35000) > once (30000).
	rent, food, once := got[0], got[1], got[2]
	if rent.CategoryID != "rent" || food.CategoryID != "food" || once.CategoryID != "once" {
		t.Fatalf("order = %s, %s, %s; want rent, food, once", rent.CategoryID, food.CategoryID, once.CategoryID)
	}
	if !equalI64(rent.Spend, []int64{50000, 50000, 100000}) || rent.Total != 200000 {
		t.Errorf("rent series = %v total %d; want [50000 50000 100000] / 200000", rent.Spend, rent.Total)
	}
	// First→last change: 100000 vs 50000 = +100%.
	if !rent.HasDelta || rent.DeltaPct != 100 {
		t.Errorf("rent delta = %d (has=%v); want +100", rent.DeltaPct, rent.HasDelta)
	}
	if !equalI64(food.Spend, []int64{10000, 20000, 5000}) || food.Total != 35000 {
		t.Errorf("food series = %v total %d; want [10000 20000 5000] / 35000", food.Spend, food.Total)
	}
	// First→last change: 5000 vs 10000 = -50%.
	if !food.HasDelta || food.DeltaPct != -50 {
		t.Errorf("food delta = %d (has=%v); want -50", food.DeltaPct, food.HasDelta)
	}
	if !equalI64(once.Spend, []int64{0, 30000, 0}) || once.Total != 30000 {
		t.Errorf("once series = %v total %d; want [0 30000 0] / 30000", once.Spend, once.Total)
	}
	// First and last buckets are both zero — no meaningful percent change.
	if once.HasDelta {
		t.Errorf("once should have no delta (first and last are 0), got %d", once.DeltaPct)
	}
}

func TestCategoryTrendsTooFewBounds(t *testing.T) {
	got, err := CategoryTrends(nil, []time.Time{dt(2026, time.June, 1)}, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("fewer than two bounds should yield no trends, got %+v", got)
	}
}

// equalI64 reports whether two int64 slices are element-wise equal.
func equalI64(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
