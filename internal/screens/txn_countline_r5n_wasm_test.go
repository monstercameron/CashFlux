// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestTxnCountSearchWidenIsATrailingStandaloneAction(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	uistate.PersistTxFilter(txnfilter.Criteria{
		Text: "coffee", From: "2026-10-01", To: "2026-10-31", PeriodPreset: txnfilter.PeriodThisMonth,
	})
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 1, Total: 319, Net: money.New(-675, "USD")}))
	line := f.Text()
	searchAt := strings.Index(line, "Search all dates (")
	if searchAt < 0 {
		t.Fatalf("count line has no all-dates action: %q", line)
	}
	for _, preceding := range []string{"1 transaction", "Spent"} {
		if at := strings.Index(line, preceding); at < 0 || at > searchAt {
			t.Errorf("%q should precede the trailing search action: %q", preceding, line)
		}
	}
	button := r2aByTestID(f, "button", "txn-search-all-dates")
	if button == nil || !strings.HasPrefix(button.Text(), "Search all dates (") {
		t.Fatalf("search action is not a concise standalone link-style button: %v", button)
	}
	if strings.Contains(button.Text(), "1 in October") || strings.Contains(button.Text(), "1 transaction") {
		t.Fatalf("search action repeats the count sentence: %q", button.Text())
	}
}

func TestTxnCountShowsNetOnlyWhenBothDirectionsHaveMoney(t *testing.T) {
	tests := []struct {
		name          string
		spent, income int64
		want          bool
	}{
		{name: "spent only", spent: 675},
		{name: "income only", income: 675},
		{name: "neither", want: false},
		{name: "both directions", spent: 675, income: 900, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnCountShowsNet(money.New(tt.spent, "USD"), money.New(tt.income, "USD")); got != tt.want {
				t.Fatalf("show net = %v, want %v", got, tt.want)
			}
		})
	}
}
