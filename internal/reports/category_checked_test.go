// SPDX-License-Identifier: MIT

package reports

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSpendingByCategoryCheckedUsesExplicitRefundsSplitsAndBounds(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rows := []domain.Transaction{
		{ID: "buy", Date: start, CategoryID: "food", Amount: money.New(-10000, "USD")},
		{ID: "refund", Date: end, CategoryID: "food", Amount: money.New(4000, "USD")},
		{ID: "split", Date: start, Amount: money.New(-7000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: money.New(-5000, "USD")}, {CategoryID: "", Amount: money.New(-3000, "USD")}, {CategoryID: "credit", Amount: money.New(1000, "USD")},
		}},
		{ID: "foreign", Date: start, CategoryID: "food", Amount: money.New(-500, "EUR")},
		{ID: "end", Date: end, CategoryID: "food", Amount: money.New(-999, "USD")},
		{ID: "excluded", Date: start, CategoryID: "food", ExcludeFromReports: true, Amount: money.New(-999, "USD")},
		{ID: "transfer", Date: start, CategoryID: "food", TransferAccountID: "other", Amount: money.New(-999, "USD")},
	}
	links := []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}}}
	netted := budgeting.NewNettedLedger(rows, links)
	// A legacy report adapter from another dataset must not influence this call.
	old := refundLinks
	refundLinks = []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(1, "USD")}}
	t.Cleanup(func() { refundLinks = old })
	got, err := SpendingByCategoryChecked(netted, start, end, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].CategoryID != "food" || got[0].Amount != 12000 || got[1].CategoryID != "" || got[1].Amount != 3000 {
		t.Fatalf("rows=%+v", got)
	}
	if rows[0].Amount.Amount != -10000 || netted[0].Amount.Amount != -6000 {
		t.Fatal("refund ledger mutated input or ignored the explicit pair")
	}
}

func TestSpendingByCategoryCheckedPreservesUnavailableFXAndArithmetic(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	for _, tc := range []struct {
		name     string
		amounts  []money.Money
		overflow bool
	}{
		{"missing FX", []money.Money{money.New(-1, "EUR")}, false},
		{"magnitude", []money.Money{money.New(math.MinInt64, "USD")}, true},
		{"sum", []money.Money{money.New(-math.MaxInt64, "USD"), money.New(-1, "USD")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var txns budgeting.NettedLedger
			for _, amount := range tc.amounts {
				txns = append(txns, domain.Transaction{Date: start, CategoryID: "food", Amount: amount})
			}
			got, err := SpendingByCategoryChecked(txns, start, end, currency.Rates{Base: "USD"})
			if err == nil || got != nil {
				t.Fatalf("unknown became valid rows=%v err=%v", got, err)
			}
			if tc.overflow && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("lost overflow: %v", err)
			}
		})
	}
}
