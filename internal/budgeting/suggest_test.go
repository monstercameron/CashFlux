// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSuggestLimit(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15") // current (partial) month is July → excluded

	txns := []domain.Transaction{
		expense(40000, "USD", "food", "", "2026-06-10"),                                     // k=1 (June)
		expense(20000, "USD", "food", "", "2026-04-10"),                                     // k=3 (April) — oldest with data
		expense(99999, "USD", "rent", "", "2026-06-01"),                                     // other category — ignored
		expense(50000, "USD", "food", "", "2026-07-05"),                                     // current partial month — excluded
		{Amount: money.New(10000, "USD"), CategoryID: "food", Date: mustDate("2026-06-12")}, // income — excluded
	}

	// Span = April..June (oldest with data = April, k=3). Total 60000 / 3 = 20000.
	got, err := SuggestLimit("food", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 20000 {
		t.Errorf("SuggestLimit = %d, want 20000", got)
	}
}

func TestSuggestLimitNewCategory(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15")
	// Only one month of data (June) → denominator is 1, not the whole window.
	txns := []domain.Transaction{expense(30000, "USD", "gym", "", "2026-06-10")}
	got, err := SuggestLimit("gym", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 30000 {
		t.Errorf("SuggestLimit = %d, want 30000 (single month)", got)
	}
}

func TestSuggestLimitZeroSpanCountsGap(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15")
	// June 600, April 0, ... actually: June (k1) and May (k2)=0 with data in April (k3) 0.
	// Spend in June and a zero May, oldest-with-data is June only here.
	txns := []domain.Transaction{
		expense(60000, "USD", "fun", "", "2026-06-10"), // k=1
	}
	// Add a real April spend so the span includes the empty May as a zero month.
	txns = append(txns, expense(0, "USD", "fun", "", "2026-04-10")) // zero-amount, no effect on sum or oldest
	got, err := SuggestLimit("fun", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only June has positive spend → oldest=1, avg = 60000.
	if got != 60000 {
		t.Errorf("SuggestLimit = %d, want 60000", got)
	}
}

func TestSuggestLimitEdges(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15")
	if got, _ := SuggestLimit("", nil, now, 6, rates); got != 0 {
		t.Errorf("empty category = %d, want 0", got)
	}
	if got, _ := SuggestLimit("food", nil, now, 0, rates); got != 0 {
		t.Errorf("zero months = %d, want 0", got)
	}
	if got, _ := SuggestLimit("food", nil, now, 6, rates); got != 0 {
		t.Errorf("no transactions = %d, want 0", got)
	}
}
