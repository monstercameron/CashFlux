// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestVisibleTransferSummaryRequiresMatchingAmounts(t *testing.T) {
	date := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", Amount: money.New(-50000, "USD"), Date: date},
		{ID: "unrelated-in", AccountID: "savings", TransferAccountID: "checking", Amount: money.New(20000, "USD"), Date: date.AddDate(0, 0, 1)},
	}
	got := SummarizeVisibleTransfers(rows)
	if got.UnpairedRows != 2 {
		t.Fatalf("unpaired = %d, want both unmatched legs", got.UnpairedRows)
	}
	if !got.CountedRows[0] || !got.CountedRows[1] {
		t.Fatalf("counted rows = %v, want both real transactions preserved", got.CountedRows)
	}
}

func TestVisibleTransferSummaryMatchesConvertedAmounts(t *testing.T) {
	date := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", Amount: money.New(-10000, "EUR"), Date: date},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", Amount: money.New(12500, "USD"), Date: date.AddDate(0, 0, 1)},
	}
	got := SummarizeVisibleTransfers(rows, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}})
	if got.UnpairedRows != 0 || !got.CountedRows[0] || got.CountedRows[1] {
		t.Fatalf("summary = %+v, want the equal converted amounts paired once", got)
	}
}
