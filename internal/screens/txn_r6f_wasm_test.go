// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR6FScheduledGhostShowsCategoryAccountAndStatusPill(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "loan", Name: "Loan from Priya's parents", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-loan", Name: "Family loan", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutRecurring(domain.Recurring{ID: "recur-loan", Label: "Family loan payment", CategoryID: "cat-loan", Amount: money.New(-12500, "USD"), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	rows := txnScheduledGhostRows([]bills.Bill{{Name: "Family loan payment", AccountID: "loan", Amount: money.New(12500, "USD"), DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}, func(time.Time) string { return "Nov 1" })
	f := render.New(t)
	f.Render(Div(rows...))
	row := r2aByTestID(f, "div", "txn-scheduled-ghost")
	if row == nil {
		t.Fatal("scheduled row was not rendered")
	}
	if !strings.Contains(row.Text(), "Family loan") || !strings.Contains(row.Text(), "Scheduled") {
		t.Fatalf("category or scheduled pill missing from row: %q", row.Text())
	}
	amount := r6fGhostCell(t, f, "−$125.00")
	if amount == nil || !strings.Contains(amount.Attr("class"), "text-down") {
		t.Fatalf("scheduled amount cell = %v, want −$125.00 in negative tone", amount)
	}
	account := r6fGhostCell(t, f, "—")
	if account.Attr("title") != "" || !strings.Contains(account.Attr("class"), "txn-scheduled-account") {
		t.Fatalf("loan account cell = %#v, want no pay-from account", account)
	}
	category := r6fGhostCell(t, f, "Loan payment")
	if !strings.Contains(category.Attr("class"), "td-cat") {
		t.Fatalf("loan category cell = %#v, want Loan payment", category)
	}
	pillFound := false
	for _, pill := range f.AllByTag("span") {
		if pill.Text() == "Scheduled" && strings.Contains(pill.Attr("class"), "txn-scheduled-pill") {
			pillFound = true
		}
	}
	if !pillFound {
		t.Fatal("status does not use the Scheduled pill")
	}
	if len(f.AllByTag("input")) != 0 || r2aByTestID(f, "button", "txn-rowedit") != nil {
		t.Fatal("scheduled row unexpectedly exposes posted-row selection or editing")
	}
	if disabled := r2aByTestID(f, "button", "txn-scheduled-edit-disabled"); disabled == nil || disabled.Attr("disabled") == "" {
		t.Fatal("scheduled row should keep the posted-row pencil slot as a disabled control")
	}
	menu := r2aByTestID(f, "button", "txn-scheduled-menu-toggle")
	if menu == nil {
		t.Fatal("scheduled row does not have its overflow menu")
	}
	menu.Click()
	open := r2aByTestID(f, "a", "txn-scheduled-open")
	if open == nil || open.Text() != "Open in Bills & repeating payments" || open.Attr("href") != "/recurring" {
		t.Fatalf("scheduled row menu item = %v, want the bills and repeating payments route", open)
	}
}

func r6fGhostCell(t *testing.T, f *render.Fixture, text string) *render.QueryNode {
	t.Helper()
	for _, node := range f.AllByTag("div") {
		if node.Attr("role") == "" && node.Text() == text {
			return node
		}
	}
	t.Fatalf("grid cell %q not found", text)
	return nil
}

func TestR6FScheduledRecurringRowShowsItsPayFromAccount(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "checking", Name: "Joint Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutRecurring(domain.Recurring{ID: "internet", Label: "Internet service", AccountID: "checking", Amount: money.New(-8500, "USD"), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	rows := txnScheduledGhostRows([]bills.Bill{{Name: "Internet service", AccountID: "recurring:internet", Amount: money.New(8500, "USD"), DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}, func(time.Time) string { return "Nov 1" })
	f := render.New(t)
	f.Render(Div(rows...))
	if cell := r6fGhostCell(t, f, "Joint Checking"); !strings.Contains(cell.Attr("class"), "txn-scheduled-account") {
		t.Fatalf("scheduled account cell = %#v, want the recurring rule's pay-from account", cell)
	}
}

func TestR9CCalendarWeekTotalsUseComparableMetrics(t *testing.T) {
	cases := []struct {
		name        string
		props       txnCalWeekTotalProps
		wantLines   []string
		wantSummary string
		wantTone    string
	}{
		{name: "spending", props: txnCalWeekTotalProps{Cashflow: txncalendar.Cashflow{Spent: 262200, Count: 4}, Base: "USD", WeekOf: "Oct 4", SummaryID: "week"}, wantLines: []string{"Spent $2,622.00", "4 transactions"}, wantSummary: "Week of Oct 4: spent $2,622.00, income $0.00, transferred $0.00, 4 transactions."},
		{name: "mixed", props: txnCalWeekTotalProps{Cashflow: txncalendar.Cashflow{Spent: 10000, Income: 25000, Moved: 5000, Count: 2}, Base: "USD", WeekOf: "Oct 4", SummaryID: "week"}, wantLines: []string{"Spent $100.00", "Income $250.00", "Transferred $50.00", "2 transactions"}, wantSummary: "Week of Oct 4: spent $100.00, income $250.00, transferred $50.00, 2 transactions.", wantTone: "text-up"},
		{name: "future due", props: txnCalWeekTotalProps{Base: "USD", Scheduled: 3, WeekOf: "Oct 25", Future: true, SummaryID: "week"}, wantLines: []string{"Spent $0.00", "3 due"}, wantSummary: "Week of Oct 25: spent $0.00, income $0.00, transferred $0.00, 0 transactions. 3 due."},
		{name: "empty future", props: txnCalWeekTotalProps{Base: "USD", WeekOf: "Oct 25", Future: true, SummaryID: "week"}, wantLines: []string{"Spent $0.00", "Nothing yet"}, wantSummary: "Week of Oct 25: spent $0.00, income $0.00, transferred $0.00, 0 transactions."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(txnCalWeekTotal(tc.props))
			node := r2aByTestID(f, "div", "txn-cal-week-total")
			if node == nil {
				t.Fatal("week total was not rendered")
			}
			text := node.Text()
			lineOffset := 0
			for _, line := range tc.wantLines {
				index := strings.Index(text[lineOffset:], line)
				if index < 0 {
					t.Errorf("week total %q missing or out of order line %q", text, line)
					continue
				}
				lineOffset += index + len(line)
			}
			if node.Attr("aria-label") != "" || node.Attr("aria-describedby") != "week" {
				t.Errorf("week total aria-label/describedby = %q / %q, want content name and summary reference", node.Attr("aria-label"), node.Attr("aria-describedby"))
			}
			foundSummary := false
			for _, span := range f.AllByTag("span") {
				if span.Attr("id") == "week" {
					foundSummary = span.Text() == tc.wantSummary && strings.Contains(span.Attr("class"), "sr-only")
				}
			}
			if !foundSummary {
				t.Errorf("hidden week summary %q not found", tc.wantSummary)
			}
			if node.Attr("style") != "" {
				t.Errorf("week total has inline style %q", node.Attr("style"))
			}
			if tc.wantTone != "" {
				foundTone := false
				for _, span := range f.AllByTag("span") {
					if strings.Contains(span.Attr("class"), tc.wantTone) {
						foundTone = true
					}
				}
				if !foundTone {
					t.Fatalf("expected tone %q missing", tc.wantTone)
				}
			}
			if tc.name == "empty future" {
				for _, span := range f.AllByTag("span") {
					if strings.Contains(span.Text(), "Spent $0.00") || strings.Contains(span.Text(), "Nothing yet") {
						if !strings.Contains(span.Attr("class"), "text-dim") {
							t.Errorf("empty future line %q is not dim", span.Text())
						}
					}
				}
			}
		})
	}
}

func TestR7CMonthHasOneWeekTotalCellPerRowIncludingEmptyEdgeWeeks(t *testing.T) {
	month, err := txncalendar.MonthCashflow(time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC), time.Monday, nil, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	weeks := month.Weeks
	rows := make([]any, 0, len(weeks))
	for i, week := range weeks {
		rows = append(rows, Div(Attr("class", "txn-cal-week-row"), ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{Cashflow: week.Total, Base: "USD", SummaryID: fmt.Sprintf("week-%d", i)})))
	}
	f := render.New(t)
	f.Render(Div(rows...))

	var renderedRows, totals []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-cal-week-row") {
			renderedRows = append(renderedRows, node)
		}
		if node.Attr("data-testid") == "txn-cal-week-total" {
			totals = append(totals, node)
		}
	}
	if len(renderedRows) != len(weeks) || len(totals) != len(weeks) {
		t.Fatalf("October grid has %d rows and %d week-total cells for %d projected weeks; want one total per row", len(renderedRows), len(totals), len(weeks))
	}
	if len(weeks) == 0 || weeks[0].Days[0].InMonth || weeks[len(weeks)-1].Days[6].InMonth {
		t.Fatal("October 2026 fixture must include leading and trailing partial-week days")
	}
}
