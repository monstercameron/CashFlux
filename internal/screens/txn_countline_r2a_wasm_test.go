// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestR2ACountLineUsesPeriodTotalsAndOmitsPageRange(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 14, Total: 3292, Range: "1–14 of 14",
		Net: money.New(42500, "USD"),
	}))
	got := f.Text()
	for _, want := range []string{"transactions", "14 transactions", "Spent", "Income", "Net +$425.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("count line %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "1–14 of 14") || strings.Contains(got, "3,292") {
		t.Errorf("count line contains a page range or unrelated all-ledger denominator: %q", got)
	}
	if r2aByTestID(f, "button", "txn-count-spent") == nil || r2aByTestID(f, "button", "txn-count-income") == nil {
		t.Fatal("Spent and Income totals must be interactive filters")
	}
}

func TestR5NCountLineOmitsZeroCashflowTotals(t *testing.T) {
	previous := appstate.Default
	previousFilter := uistate.CurrentTxFilter()
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("create empty count-line app: %v", err)
	}
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", PeriodPreset: txnfilter.PeriodThisMonth})
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.PersistTxFilter(previousFilter)
	})
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 14, Total: 14}))
	got := f.Text()
	if !strings.Contains(got, "14 transactions") {
		t.Fatalf("count line %q does not include the transaction count", got)
	}
	if strings.Contains(got, "Spent") || strings.Contains(got, "Income") || strings.Contains(got, "Net") {
		t.Fatalf("zero cashflow line should show only its count, got %q", got)
	}
}

func TestR3NCountLinePluralizesAndRemainsVisibleForAnEmptyPeriod(t *testing.T) {
	for _, tc := range []struct {
		shown int
		want  string
	}{{shown: 1, want: "1 transaction"}, {shown: 2, want: "2 transactions"}} {
		if got := txnCountTransactionCount(tc.shown); got != tc.want {
			t.Errorf("txnCountTransactionCount(%d) = %q, want %q", tc.shown, got, tc.want)
		}
	}
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 0, Total: 3292}))
	if node := r2aByTestID(f, "span", "txn-count-line"); node == nil || !strings.Contains(node.Text(), "0 transactions") {
		t.Fatalf("empty-period count line missing or inaccurate: %v", node)
	}
}

func r2aByTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}

func TestR2ACountLinePeriodLabels(t *testing.T) {
	tests := []struct {
		name   string
		filter txnfilter.Criteria
		want   string
	}{
		{name: "all dates", want: "All dates"},
		{name: "month", filter: txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31"}, want: "October 2026"},
		{name: "custom range", filter: txnfilter.Criteria{From: "2026-09-30", To: "2026-10-02"}, want: "Custom range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnCountPeriod(tt.filter); got != tt.want {
				t.Errorf("txnCountPeriod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestR2ASignedNetUsesAnExplicitSign(t *testing.T) {
	tests := []struct {
		amount int64
		want   string
	}{
		{amount: 12500, want: "+$125.00"},
		{amount: -12500, want: "−$125.00"},
		{amount: 0, want: "$0.00"},
	}
	for _, tt := range tests {
		if got := signedTxnMoney(money.New(tt.amount, "USD")); got != tt.want {
			t.Errorf("signedTxnMoney(%d) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}

func TestR2AUpcomingSummaryUsesSingularAndPluralBillCopy(t *testing.T) {
	if got := txnUpcomingSummary(1, "$220.00"); got != "1 bill still to come ($220.00)" {
		t.Errorf("single upcoming summary = %q", got)
	}
	if got := txnUpcomingSummary(2, "$440.00"); got != "2 bills still to come ($440.00)" {
		t.Errorf("plural upcoming summary = %q", got)
	}
}
