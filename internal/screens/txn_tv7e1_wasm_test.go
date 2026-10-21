// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestTV7ESearchAllDatesActionHidesZeroAndShowsPositive(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		want  int
	}{
		{name: "zero", count: 0, want: 0},
		{name: "positive", count: 3, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(txnSearchAllDatesAction(tc.count, nil))
			if got := r2aByTestID(f, "button", "txn-search-all-dates"); (got != nil) != (tc.want == 1) {
				t.Fatalf("search all dates action present = %t, want %t", got != nil, tc.want == 1)
			}
		})
	}
}

func TestTV7ENoMatchHintNeedsAnotherFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter txnfilter.Criteria
		want   bool
	}{
		{name: "search only", filter: txnfilter.Criteria{Text: "coffee"}},
		{name: "search and category", filter: txnfilter.Criteria{Text: "coffee", Category: "food"}, want: true},
		{name: "search and date only", filter: txnfilter.Criteria{Text: "coffee", PeriodPreset: txnfilter.PeriodThisMonth, From: "2026-10-01", To: "2026-10-31"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := txnNoMatchHasOtherFilters(tc.filter); got != tc.want {
				t.Fatalf("txnNoMatchHasOtherFilters() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestTV7EUpcomingSummaryIncludesEarliestDueDate(t *testing.T) {
	date := func(value time.Time) string { return value.Format("Jan 2") }
	items := []bills.Bill{
		{Name: "Later", Amount: money.New(20000, "USD"), DueDate: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)},
		{Name: "Earlier", Amount: money.New(22000, "USD"), DueDate: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)},
	}
	if got := txnUpcomingDueSummary(items[:1], "$200.00", date); got != "$200 due Oct 29" {
		t.Errorf("single bill summary = %q", got)
	}
	if got := txnUpcomingDueSummary(items, "$420.00", date); got != "2 bills due by Oct 22, $420 total" {
		t.Errorf("multiple bill summary = %q", got)
	}
}

func TestTV7ECounterpartNoneCopyIsPlainEnglish(t *testing.T) {
	if got := uistate.T("transactions.classifyNone"); got != "None (income or spending)" {
		t.Fatalf("counterpart None copy = %q", got)
	}
}
