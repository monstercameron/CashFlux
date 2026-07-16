// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// tagged builds a spend transaction carrying tags.
func tagged(major int64, on time.Time, tags ...string) domain.Transaction {
	return domain.Transaction{CategoryID: "c", Amount: money.New(-major*100, "USD"), Date: on, Tags: tags}
}

func TestSpendingByTag(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		tagged(100, dt(2026, time.June, 5), "Vacation", "splurge"),
		tagged(40, dt(2026, time.June, 8), "vacation"),                                                                       // case-folds into Vacation
		tagged(25, dt(2026, time.June, 9), "splurge", "SPLURGE"),                                                             // duplicate tag on one charge counts once
		tagged(999, dt(2026, time.May, 20), "vacation"),                                                                      // out of range
		{Amount: money.New(5000, "USD"), Date: dt(2026, time.June, 10), Tags: []string{"vacation"}},                          // income — excluded
		{Amount: money.New(-7000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 10), Tags: []string{"vacation"}}, // transfer — excluded
		tagged(10, dt(2026, time.June, 11)),                                                                                  // untagged — invisible here
	}
	got, err := SpendingByTag(txns, start, end, false, time.Time{}, time.Time{}, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tags, want 2 (Vacation, splurge): %+v", len(got), got)
	}
	// Vacation: 100+40 = $140 across 2 charges; first-seen casing kept.
	if got[0].Tag != "Vacation" || got[0].Amount != 14000 || got[0].Count != 2 {
		t.Errorf("row 0 = %+v, want Vacation 14000 x2", got[0])
	}
	// splurge: 100+25 = $125 across 2 charges (the doubled tag counted once).
	if got[1].Tag != "splurge" || got[1].Amount != 12500 || got[1].Count != 2 {
		t.Errorf("row 1 = %+v, want splurge 12500 x2", got[1])
	}
}

func TestSpendingByTag_PriorComparisonAndDropped(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	ps, pe := dt(2026, time.May, 1), dt(2026, time.June, 1)
	txns := []domain.Transaction{
		tagged(50, dt(2026, time.June, 5), "coffee"),
		tagged(30, dt(2026, time.May, 5), "coffee"),
		tagged(80, dt(2026, time.May, 6), "cigarettes"), // quit in June — current 0, prior 8000
	}
	got, err := SpendingByTag(txns, start, end, true, ps, pe, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tags, want 2: %+v", len(got), got)
	}
	if got[0].Tag != "coffee" || got[0].Amount != 5000 || got[0].Prior != 3000 {
		t.Errorf("coffee = %+v, want 5000 cur / 3000 prior", got[0])
	}
	if got[1].Tag != "cigarettes" || got[1].Amount != 0 || got[1].Prior != 8000 {
		t.Errorf("cigarettes = %+v, want 0 cur / 8000 prior (dropped habit still listed)", got[1])
	}
}
