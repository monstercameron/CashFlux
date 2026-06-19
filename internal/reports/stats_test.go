package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSpendingStats(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		expense("a", 10, dt(2026, time.June, 2)),
		expense("b", 20, dt(2026, time.June, 3)),
		expense("c", 300, dt(2026, time.June, 4)),                                               // big purchase skews the mean
		expense("d", 9999, dt(2026, time.May, 30)),                                              // out of range
		{Amount: money.New(5000, "USD"), Date: dt(2026, time.June, 5)},                          // income excluded
		{Amount: money.New(-7000, "USD"), TransferAccountID: "x", Date: dt(2026, time.June, 6)}, // transfer excluded
	}
	got, err := SpendingStats(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1000 + 2000 + 30000 = 33000 over 3 → mean 11000; median 2000 (the middle).
	if got.Count != 3 || got.Total != 33000 || got.Average != 11000 || got.Median != 2000 {
		t.Errorf("stats = %+v, want count=3 total=33000 avg=11000 median=2000", got)
	}
}

func TestSpendingStatsEvenMedian(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		expense("a", 10, dt(2026, time.June, 2)),  // 1000
		expense("b", 20, dt(2026, time.June, 3)),  // 2000
		expense("c", 40, dt(2026, time.June, 4)),  // 4000
		expense("d", 100, dt(2026, time.June, 5)), // 10000
	}
	got, _ := SpendingStats(txns, start, end, usdRates())
	// median of {1000,2000,4000,10000} = (2000+4000)/2 = 3000.
	if got.Median != 3000 {
		t.Errorf("median = %d, want 3000", got.Median)
	}
}

func TestSpendingStatsEmpty(t *testing.T) {
	got, err := SpendingStats(nil, dt(2026, time.June, 1), dt(2026, time.July, 1), usdRates())
	if err != nil || got != (SpendStats{}) {
		t.Errorf("empty = %+v err=%v, want zero value", got, err)
	}
}
