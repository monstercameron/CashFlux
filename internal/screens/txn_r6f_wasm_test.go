// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
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
	amount := r6fGridCell(t, f, "−$125.00")
	if amount == nil || !strings.Contains(amount.Attr("class"), "text-down") {
		t.Fatalf("scheduled amount cell = %v, want −$125.00 in negative tone", amount)
	}
	account := r6fGridCell(t, f, "Loan from Priya's parents")
	if account.Attr("title") != "Loan from Priya's parents" || !strings.Contains(account.Attr("class"), "txn-scheduled-account") {
		t.Fatalf("account cell lacks single-line tooltip marker: %#v", account)
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
}

func r6fGridCell(t *testing.T, f *render.Fixture, text string) *render.QueryNode {
	t.Helper()
	for _, node := range f.AllByTag("div") {
		if node.Attr("role") == "gridcell" && node.Text() == text {
			return node
		}
	}
	t.Fatalf("grid cell %q not found", text)
	return nil
}

func TestR9CCalendarWeekTotalsUseComparableMetrics(t *testing.T) {
	cases := []struct {
		name      string
		props     txnCalWeekTotalProps
		wantLines []string
		wantAria  string
		wantTone  string
	}{
		{name: "spending", props: txnCalWeekTotalProps{Stat: txncalendar.DayStat{Net: -262200, Count: 4}, Base: "USD", Spent: 262200, WeekOf: "Oct 4"}, wantLines: []string{"Money out −$2,622.00", "4 transactions"}, wantAria: "Week of Oct 4: money out $2,622.00, 4 transactions."},
		{name: "mixed", props: txnCalWeekTotalProps{Stat: txncalendar.DayStat{Net: 15000, Count: 2}, Base: "USD", Spent: 10000, Income: 25000, WeekOf: "Oct 4"}, wantLines: []string{"Money out −$100.00", "Money in $250.00", "2 transactions"}, wantAria: "Week of Oct 4: money out $100.00, money in $250.00, 2 transactions.", wantTone: "text-up"},
		{name: "future scheduled", props: txnCalWeekTotalProps{Base: "USD", Scheduled: 3, WeekOf: "Oct 25", Future: true}, wantLines: []string{"Money out $0.00", "3 scheduled"}, wantAria: "Week of Oct 25: money out $0.00, 3 scheduled."},
		{name: "empty future", props: txnCalWeekTotalProps{Base: "USD", WeekOf: "Oct 25", Future: true}, wantLines: []string{"Money out $0.00", "Nothing yet"}, wantAria: "Week of Oct 25: money out $0.00, Nothing yet."},
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
			if node.Attr("aria-label") != tc.wantAria || node.Attr("title") != tc.wantAria {
				t.Errorf("week total aria/title = %q / %q, want %q", node.Attr("aria-label"), node.Attr("title"), tc.wantAria)
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

func TestR9CMixedWeekAmountsSeparateIncomeFromSpending(t *testing.T) {
	week := []txncalendar.Cell{
		{Date: time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC), InMonth: true},
		{Date: time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC), InMonth: true},
	}
	txns := []domain.Transaction{
		{Date: week[0].Date, Amount: money.New(-1250, "USD")},
		{Date: week[1].Date, Amount: money.New(2500, "USD")},
		{Date: week[1].Date, Amount: money.New(-9000, "USD"), TransferAccountID: "savings"},
	}
	spent, income := calWeekAmounts(week, txns, currency.Rates{Base: "USD"})
	if spent != 1250 || income != 2500 {
		t.Fatalf("week cashflow = spent %d income %d, want 1250/2500", spent, income)
	}
}

func TestR7CMonthHasOneWeekTotalCellPerRowIncludingEmptyEdgeWeeks(t *testing.T) {
	weeks, err := txncalendar.MonthInBase(time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC), time.Monday, nil, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]any, 0, len(weeks))
	for _, week := range weeks {
		rows = append(rows, Div(Attr("class", "txn-cal-week-row"), ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{Stat: txncalendar.WeekStat(week), Base: "USD"})))
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
	if len(weeks) == 0 || weeks[0][0].InMonth || weeks[len(weeks)-1][6].InMonth {
		t.Fatal("October 2026 fixture must include leading and trailing partial-week days")
	}
}
