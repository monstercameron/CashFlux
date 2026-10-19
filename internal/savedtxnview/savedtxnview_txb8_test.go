// SPDX-License-Identifier: MIT

package savedtxnview

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestTXB8MissingFXRateIsNotAddedAsBaseCurrency(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "usd", Amount: money.New(1000, "USD")},
		{ID: "eur", Amount: money.New(800, "EUR")},
	}
	rates := currency.Rates{Base: "USD"}
	view := SavedTxnView{Criteria: txnfilter.Criteria{}}

	count, total, unconverted := view.SummaryConverted(txns, func(tx domain.Transaction) (int64, bool) {
		converted, err := rates.Convert(tx.Amount, "USD")
		if err != nil {
			return 0, false
		}
		return converted.Amount, true
	})
	if count != 2 {
		t.Fatalf("Summary count = %d, want 2 matching rows", count)
	}
	if total != 1000 {
		t.Fatalf("Summary total = %d, want 1000 USD minor units; unconvertible EUR must be excluded", total)
	}
	if unconverted != 1 {
		t.Fatalf("Summary unconverted = %d, want 1 excluded EUR row", unconverted)
	}
}
