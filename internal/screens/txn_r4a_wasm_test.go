// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestR4APageFilterBadgeChipAndClearCountsAgree(t *testing.T) {
	c := txnfilter.Criteria{Text: "coffee", Category: "food", Flow: "out", From: "2026-10-01", To: "2026-10-31"}
	pageFilters := c.PageFilters()
	chips := txnActiveFilterChips(pageFilters, func(f txnfilter.ActiveFilter) string { return f.Value })
	if len(pageFilters) != 3 || len(chips) != len(pageFilters) || clearAllLabel(len(pageFilters)) != clearAllLabel(len(chips)) {
		t.Fatalf("page filters/chips/clear count disagree: %d / %d / %q", len(pageFilters), len(chips), clearAllLabel(len(pageFilters)))
	}
	cleared := c.ClearPageFilters()
	if cleared.From != c.From || cleared.To != c.To {
		t.Fatalf("clear all lost the selected period: %+v", cleared)
	}
}

func TestR4ASearchAllDatesLinkWidensOnlyOnClick(t *testing.T) {
	uistate.PersistTxFilter(txnfilter.Criteria{Text: "coffee", From: "2026-10-01", To: "2026-10-31", PeriodPreset: txnfilter.PeriodThisMonth})
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 1, Total: 319, Net: money.New(0, "USD"),
		Spent: money.New(100, "USD"), Income: money.New(100, "USD"), CashflowProvided: true}))
	var link *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "txn-search-all-dates" {
			link = button
		}
	}
	if link == nil || !strings.Contains(link.Text(), "Search all dates (0)") {
		t.Fatalf("search widen action = %v, want an explicit all-dates link", link)
	}
	if got := uistate.CurrentTxFilter().PeriodPreset; got != txnfilter.PeriodThisMonth {
		t.Fatalf("rendering the search link silently changed period to %q", got)
	}
	link.Click()
	if got := uistate.CurrentTxFilter(); got.From != "" || got.To != "" || got.PeriodPreset != txnfilter.PeriodAllDates {
		t.Fatalf("click did not widen to all dates: %+v", got)
	}
}

func TestR4ASpentAndIncomeToggleWithPressedState(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", PeriodPreset: txnfilter.PeriodThisMonth})
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 2, Total: 10,
		Spent: money.New(100, "USD"), Income: money.New(100, "USD"), CashflowProvided: true}))
	spent := r2aByTestID(f, "button", "txn-count-spent")
	income := r2aByTestID(f, "button", "txn-count-income")
	if spent == nil || income == nil || spent.Attr("aria-pressed") != "false" || income.Attr("aria-pressed") != "false" {
		t.Fatalf("initial direction controls lack unpressed state: spent=%v income=%v", spent, income)
	}
	spent.Click()
	f.Stabilize()
	spent = r2aByTestID(f, "button", "txn-count-spent")
	if spent == nil || spent.Attr("aria-pressed") != "true" || uistate.CurrentTxFilter().Flow != "out" {
		t.Fatal("Spent did not set its pressed state and expense filter")
	}
	spent.Click()
	if uistate.CurrentTxFilter().Flow != "" {
		t.Fatal("clicking Spent a second time did not clear its direction filter")
	}
}

func TestR4AFutureScheduledRowsUseLedgerCells(t *testing.T) {
	rows := txnScheduledGhostRows([]bills.Bill{{Name: "Rent", AccountID: "checking", Amount: money.New(120000, "USD"), DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}, func(time.Time) string { return "Nov 1" })
	f := render.New(t)
	f.Render(Div(rows...))
	ghosts := f.AllByTag("div")
	found := false
	for _, row := range ghosts {
		if row.Attr("data-testid") == "txn-scheduled-ghost" {
			found = true
			if row.Attr("role") != "" || strings.Count(row.Text(), "Rent") != 1 || !strings.Contains(row.Text(), "Scheduled") || !strings.Contains(row.Text(), "$1,200.00") {
				t.Fatalf("scheduled ghost row does not read like a ledger row: %q", row.Text())
			}
			cells := 0
			for _, child := range f.AllByTag("div") {
				if child.Attr("role") == "gridcell" || child.Attr("role") == "row" {
					cells++
				}
			}
			if cells != 0 {
				t.Fatalf("ghost row has %d orphan grid roles, want none", cells)
			}
		}
	}
	if !found {
		t.Fatal("future scheduled row was not rendered")
	}
}

func TestR4AFutureCountLineStaysVisibleWithoutPostedRows(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	period := txnfilter.Criteria{From: "2026-11-01", To: "2026-11-30", PeriodPreset: txnfilter.MonthPeriodPreset(2026, time.November)}
	uistate.PersistTxFilter(period)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 0, Total: 0}))
	line := r2aByTestID(f, "span", "txn-count-line")
	if line == nil || !strings.Contains(line.Text(), "0 posted. 0 scheduled. Due $0.00.") {
		if line == nil {
			t.Fatal("future period count line is missing")
		}
		t.Fatalf("future period count line = %q, want posted/scheduled summary", line.Text())
	}
	main := r2aByTestID(f, "span", "txn-count-line")
	if main == nil {
		t.Fatal("future period count line is missing")
	}
	var summary, amount *render.QueryNode
	for _, span := range f.AllByTag("span") {
		if strings.Contains(span.Attr("class"), "txn-count-main") {
			summary = span
		}
		if span.Text() == "Due $0.00" {
			amount = span
		}
	}
	if summary == nil || strings.Contains(summary.Attr("class"), "text-down") {
		t.Fatalf("future count summary class = %v, want normal count-line color", summary)
	}
	if amount == nil {
		t.Fatal("future expected-money amount is missing from the count line")
	}
}
