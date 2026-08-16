// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

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

// --- C586: the estimate must describe the same scope the budget's card does ---

// A parent-category budget's card rolls up its sub-categories; an estimate that
// counted only charges booked directly to the parent produced a second, smaller
// number for the same budget.
func TestSuggestLimitInCoversDescendants(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		{ID: "1", CategoryID: "gas", Amount: money.New(-6000, "USD"), Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},
		{ID: "2", CategoryID: "autoloan", Amount: money.New(-4000, "USD"), Date: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)},
		{ID: "3", CategoryID: "transport", Amount: money.New(-1000, "USD"), Date: time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)},
	}

	parentOnly, err := SuggestLimit("transport", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("SuggestLimit: %v", err)
	}
	if parentOnly != 1000 {
		t.Errorf("parent-only estimate = %d, want 1000", parentOnly)
	}

	scoped, err := SuggestLimitIn(map[string]bool{"transport": true, "gas": true, "autoloan": true}, txns, now, 6, rates)
	if err != nil {
		t.Fatalf("SuggestLimitIn: %v", err)
	}
	if scoped != 11000 {
		t.Errorf("scoped estimate = %d, want 11000 (the whole rollup, matching the card)", scoped)
	}
}

// A split receipt contributes each LINE to its own category, exactly as
// budgeting.spentCovered counts it — never the whole charge to the parent.
func TestSuggestLimitInAttributesSplitLines(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{{
		ID: "receipt", CategoryID: "shopping", Amount: money.New(-12000, "USD"),
		Date: time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC),
		Splits: []domain.CategorySplit{
			{CategoryID: "groceries", Amount: money.New(-8000, "USD")},
			{CategoryID: "household", Amount: money.New(-4000, "USD")},
		},
	}}

	got, err := SuggestLimitIn(map[string]bool{"groceries": true}, txns, now, 6, rates)
	if err != nil {
		t.Fatalf("SuggestLimitIn: %v", err)
	}
	if got != 8000 {
		t.Errorf("estimate = %d, want 8000 (only the groceries line)", got)
	}
}

// A category with no history has no estimate to give — 0, so the form can say
// "no history yet" instead of borrowing another category's average (C586).
func TestSuggestLimitInBrandNewCategory(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		{ID: "1", CategoryID: "transport", Amount: money.New(-110000, "USD"), Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},
	}
	got, err := SuggestLimitIn(map[string]bool{"petcare": true}, txns, now, 6, rates)
	if err != nil {
		t.Fatalf("SuggestLimitIn: %v", err)
	}
	if got != 0 {
		t.Errorf("estimate for a brand-new category = %d, want 0", got)
	}
}
