package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSavingsRateSeries(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.April, 1),
		dt(2026, time.May, 1),
		dt(2026, time.June, 1),
	}
	txns := []domain.Transaction{
		// April: income 1000, expense 500 → kept 50%.
		income(1000, dt(2026, time.April, 10)),
		payeeExpense("x", 500, dt(2026, time.April, 12)),
		// May: income 1000, expense 1500 → overspent, -50%.
		income(1000, dt(2026, time.May, 5)),
		payeeExpense("y", 1500, dt(2026, time.May, 15)),
	}
	got, err := SavingsRateSeries(txns, bounds, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got))
	}
	if got[0] != 50 {
		t.Errorf("April rate = %d, want 50", got[0])
	}
	if got[1] != -50 {
		t.Errorf("May rate = %d, want -50", got[1])
	}
}

func TestSavingsRateSeriesNoIncome(t *testing.T) {
	bounds := []time.Time{dt(2026, time.June, 1), dt(2026, time.July, 1)}
	txns := []domain.Transaction{payeeExpense("x", 100, dt(2026, time.June, 5))}
	got, err := SavingsRateSeries(txns, bounds, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No income → rate 0 (no meaningful baseline).
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("got %v, want [0]", got)
	}
}
