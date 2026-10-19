// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSearchTagWithDisplayedHash(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "exact", Tags: []string{"coffee-run"}},
		{ID: "prefix", Tags: []string{"coffee-run-2026"}},
		{ID: "other", Tags: []string{"run-coffee"}},
	}
	if got := ids(Apply(txns, Criteria{Text: "#coffee-run"})); got != "exactprefix" {
		t.Fatalf("displayed tag search = %q, want exact and prefix tag matches", got)
	}
}

func TestDirectionFiltersExcludeTransfers(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "income", Amount: money.New(675, "USD")},
		{ID: "expense", Amount: money.New(-675, "USD")},
		{ID: "in-transfer", TransferAccountID: "mortgage", Amount: money.New(675, "USD")},
		{ID: "out-transfer", TransferAccountID: "checking", Amount: money.New(-675, "USD")},
	}
	if got := ids(Apply(txns, Criteria{Flow: "in"})); got != "income" {
		t.Errorf("Money in filter = %q, want only income", got)
	}
	if got := ids(Apply(txns, Criteria{Flow: "out"})); got != "expense" {
		t.Errorf("Money out filter = %q, want only expense", got)
	}
}

func TestSearchExactAmount(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "coffee", Payee: "Cafe", Amount: money.New(-675, "USD")},
		{ID: "income", Payee: "Reimbursement", Amount: money.New(675, "USD")},
		{ID: "nearby", Payee: "Cafe", Amount: money.New(-676, "USD")},
	}
	for _, query := range []string{"6.75", "$6.75"} {
		if got := ids(Apply(txns, Criteria{Text: query})); got != "coffeeincome" {
			t.Errorf("amount search %q = %q, want exact matches regardless of sign", query, got)
		}
	}
}
