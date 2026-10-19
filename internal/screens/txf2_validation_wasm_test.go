// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestTxnFilterRangeValidation(t *testing.T) {
	tests := []struct {
		name   string
		filter uistate.TxFilter
		want   string
	}{
		{name: "amount bounds ordered", filter: uistate.TxFilter{AmountMin: "100", AmountMax: "500"}},
		{name: "amount bounds inverted", filter: uistate.TxFilter{AmountMin: "500", AmountMax: "100"}, want: "Minimum is larger than maximum."},
		{name: "date bounds ordered", filter: uistate.TxFilter{From: "2026-10-01", To: "2026-10-31"}},
		{name: "date bounds inverted", filter: uistate.TxFilter{From: "2026-10-31", To: "2026-10-01"}, want: "Start date is after end date."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnFilterRangeError(tt.filter, "USD"); got != tt.want {
				t.Fatalf("range error = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTxnAmountChipNamesFieldAndCurrency(t *testing.T) {
	if got, want := txnAmountChipLabel("min", "500", "USD"), "Amount ≥ $500.00"; got != want {
		t.Fatalf("minimum chip = %q, want %q", got, want)
	}
	if got, want := txnAmountChipLabel("max", "100", "USD"), "Amount ≤ $100.00"; got != want {
		t.Fatalf("maximum chip = %q, want %q", got, want)
	}
	if got, want := txnAmountChipLabel("min", "500", "JPY"), "Amount ≥ ¥500"; got != want {
		t.Fatalf("zero-decimal currency chip = %q, want %q", got, want)
	}
}

func TestTxnCountLineKeepsNoMatchMessageOnlyInCard(t *testing.T) {
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 0, Total: 3, Net: money.Zero("USD")}))
	if got := f.Text(); !strings.Contains(got, "0 transactions") {
		t.Fatalf("no-match count line text = %q, want visible zero count", got)
	}
}

func TestTxnCountLineNamesExcludedRows(t *testing.T) {
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 3, Total: 3, Net: money.New(8500, "USD"), Excluded: 1}))
	got := f.Text()
	if !strings.Contains(got, "3 transactions") {
		t.Errorf("count line = %q, want transaction count", got)
	}
	secondary := byTestID(f, "div", "txn-count-secondary")
	if secondary == nil || !strings.Contains(secondary.Text(), "left out of budgets") {
		t.Errorf("secondary facts = %v, want excluded rows named", secondary)
	}
	if !strings.Contains(got, "left out of budgets") {
		t.Errorf("accessible summary = %q, want excluded rows named", got)
	}
}

func TestBulkCategoryChoiceConfirmsCategoryWithoutChangingIngestionSource(t *testing.T) {
	got := txnCategoryAfterManualChoice(domain.Transaction{Source: domain.TxnSourceImported})
	if got.Source != domain.TxnSourceImported || !got.Reviewed {
		t.Fatalf("bulk category decision = %#v, want imported source preserved and category confirmed", got)
	}
}
