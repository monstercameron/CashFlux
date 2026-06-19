package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSpendingAnomalies(t *testing.T) {
	now := dt(2026, time.July, 15) // current month = July; trailing = Apr/May/Jun
	txns := []domain.Transaction{
		// Dining: $100/mo trailing, then $300 this month → 200% over → flagged.
		expense("dining", 100, dt(2026, time.April, 5)),
		expense("dining", 100, dt(2026, time.May, 5)),
		expense("dining", 100, dt(2026, time.June, 5)),
		expense("dining", 300, dt(2026, time.July, 5)),
		// Rent: steady $1000/mo → not flagged.
		expense("rent", 1000, dt(2026, time.April, 1)),
		expense("rent", 1000, dt(2026, time.May, 1)),
		expense("rent", 1000, dt(2026, time.June, 1)),
		expense("rent", 1000, dt(2026, time.July, 1)),
		// Tiny: way over its norm but below the absolute floor → skipped.
		expense("gum", 1, dt(2026, time.June, 3)),
		expense("gum", 40, dt(2026, time.July, 3)),
		// New: only this month, no baseline → skipped.
		expense("new", 500, dt(2026, time.July, 8)),
	}
	got, err := SpendingAnomalies(txns, now, 3, 50, 5000, usdRates()) // overPct 50, floor $50
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d anomalies, want 1 (dining): %+v", len(got), got)
	}
	a := got[0]
	if a.CategoryID != "dining" {
		t.Errorf("category = %q, want dining", a.CategoryID)
	}
	if a.Current != 30000 || a.Average != 10000 {
		t.Errorf("current/average = %d/%d, want 30000/10000", a.Current, a.Average)
	}
	if a.OverPct != 200 {
		t.Errorf("OverPct = %d, want 200", a.OverPct)
	}
}

func TestSpendingAnomaliesNoneWhenSteady(t *testing.T) {
	now := dt(2026, time.July, 15)
	txns := []domain.Transaction{
		expense("rent", 1000, dt(2026, time.May, 1)),
		expense("rent", 1000, dt(2026, time.June, 1)),
		expense("rent", 1000, dt(2026, time.July, 1)),
	}
	got, err := SpendingAnomalies(txns, now, 3, 50, 1000, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("steady spending should flag nothing, got %+v", got)
	}
}

func TestSpendingAnomaliesEdges(t *testing.T) {
	now := dt(2026, time.July, 15)
	if got, _ := SpendingAnomalies(nil, now, 0, 50, 0, usdRates()); got != nil {
		t.Errorf("zero months should yield nil, got %+v", got)
	}
	if got, _ := SpendingAnomalies(nil, now, 3, 50, 0, usdRates()); len(got) != 0 {
		t.Errorf("no transactions should yield none, got %+v", got)
	}
}
