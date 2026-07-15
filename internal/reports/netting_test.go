// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestRefundNettingPeriodTrue proves the XC2 read-model contract for reports: a
// $120 clothing purchase in March returned $40 in April nets in MARCH, and April
// shows neither inflated income nor a phantom negative.
func TestRefundNettingPeriodTrue(t *testing.T) {
	buy := domain.Transaction{ID: "buy", Amount: money.New(-12000, "USD"), CategoryID: "clothing", Date: dt(2026, time.March, 10)}
	refund := domain.Transaction{ID: "ref", Amount: money.New(4000, "USD"), CategoryID: "clothing", Date: dt(2026, time.April, 5)}
	txns := []domain.Transaction{buy, refund}
	links := []domain.TxnLink{{ID: "p", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "ref"}, Amount: money.New(4000, "USD")}}

	marchStart, marchEnd := dt(2026, time.March, 1), dt(2026, time.April, 1)
	aprStart, aprEnd := dt(2026, time.April, 1), dt(2026, time.May, 1)

	SetRefundLinks(links)
	defer SetRefundLinks(nil)

	// IncomeVsExpense: March = $80 net expense, no income.
	mar, err := IncomeVsExpense(txns, marchStart, marchEnd, usdRates())
	if err != nil {
		t.Fatalf("March IncomeVsExpense: %v", err)
	}
	if mar.Expense != 8000 || mar.Income != 0 {
		t.Errorf("March income/expense = %d/%d, want 0/8000", mar.Income, mar.Expense)
	}

	// April: the refund is zeroed — no phantom income, no phantom negative.
	apr, err := IncomeVsExpense(txns, aprStart, aprEnd, usdRates())
	if err != nil {
		t.Fatalf("April IncomeVsExpense: %v", err)
	}
	if apr.Income != 0 || apr.Expense != 0 {
		t.Errorf("April income/expense = %d/%d, want 0/0 (no phantom)", apr.Income, apr.Expense)
	}

	// Category totals: clothing reads $80 in March, absent in April.
	marCats, err := categoryTotals(txns, marchStart, marchEnd, usdRates())
	if err != nil {
		t.Fatalf("March categoryTotals: %v", err)
	}
	if marCats["clothing"] != 8000 {
		t.Errorf("March clothing = %d, want 8000", marCats["clothing"])
	}
	aprCats, err := categoryTotals(txns, aprStart, aprEnd, usdRates())
	if err != nil {
		t.Fatalf("April categoryTotals: %v", err)
	}
	if aprCats["clothing"] != 0 {
		t.Errorf("April clothing = %d, want 0", aprCats["clothing"])
	}

	// Baseline sanity: with netting off, March is the full $120 blowout.
	SetRefundLinks(nil)
	base, _ := categoryTotals(txns, marchStart, marchEnd, usdRates())
	if base["clothing"] != 12000 {
		t.Errorf("baseline March clothing = %d, want 12000", base["clothing"])
	}
}
