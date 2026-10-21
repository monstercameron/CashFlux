// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTV7CTransactionPageCalendarToggleShowsLiabilityDueDate(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	dueDay := now.Day() + 1
	if dueDay > 28 {
		dueDay = 28
	}
	due := time.Date(now.Year(), now.Month(), dueDay, 0, 0, 0, 0, time.UTC)
	if err := app.PutAccount(domain.Account{
		ID: "tv7c-card", Name: "Rewards Credit Card", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
		DueDayOfMonth: dueDay, MinPayment: money.New(22000, "USD"),
	}); err != nil {
		t.Fatal(err)
	}
	previousApp := appstate.Default
	previousFilter := uistate.CurrentTxFilter()
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{
		From: dateutil.MonthStart(now).Format("2006-01-02"),
		To:   dateutil.AddMonths(dateutil.MonthStart(now), 1).AddDate(0, 0, -1).Format("2006-01-02"),
	})
	uistate.BumpDataRevision()
	t.Cleanup(func() {
		appstate.Default = previousApp
		uistate.PersistTxFilter(previousFilter)
	})

	f := render.New(t)
	f.Render(ui.CreateElement(Transactions))
	calendarToggle := r2aByTestID(f, "button", "txn-view-calendar")
	if calendarToggle == nil {
		t.Fatal("transaction toolbar calendar toggle is missing")
	}
	calendarToggle.Click()
	f.Stabilize()
	cell := r2aByTestID(f, "button", "txn-cal-day-"+due.Format("2006-01-02"))
	if cell == nil || !strings.Contains(cell.Text(), "Due $220.00") || !strings.Contains(cell.Text(), "Rewards Credit Card") {
		t.Fatalf("transaction page due-day cell = %v, want the scheduled $220 credit-card payment", cell)
	}
}

func TestTV7CCalendarDayShowsScheduledLiabilityAmountAndName(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-22", Day: 22, InMonth: true,
		Rates: currency.Rates{Base: "USD"},
		Ghosts: []txncalendar.Ghost{{Label: "Rewards Credit Card", Amount: -22000, Currency: "USD",
			Date: time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC), Liability: true}},
	}))
	button := f.AllByTag("button")[0]
	if got := button.Text(); !strings.Contains(got, "Due $220.00") || !strings.Contains(got, "Rewards Credit Card") {
		t.Fatalf("Oct 22 cell = %q, want due amount and liability name", got)
	}
	if got := button.Attr("aria-label"); !strings.Contains(got, "Rewards Credit Card is due on this day for $220.00") {
		t.Fatalf("Oct 22 accessible name = %q, want scheduled payment and amount", got)
	}
}

func TestTV7CEmptyCalendarWeekHidesZeroSpentAndTransactionCopy(t *testing.T) {
	f := render.New(t)
	f.Render(txnCalWeekTotal(txnCalWeekTotalProps{Base: "USD", WeekOf: "Oct 4", SummaryID: "week"}))
	node := r2aByTestID(f, "div", "txn-cal-week-total")
	if node == nil {
		t.Fatal("week total was not rendered")
	}
	var countLine bool
	for _, span := range f.AllByTag("span") {
		if strings.Contains(span.Attr("class"), "txn-cal-week-spent") {
			t.Fatalf("empty week rendered a zero-spent label: %q", span.Text())
		}
		if strings.Contains(span.Attr("class"), "txn-cal-week-count") {
			countLine = strings.TrimSpace(span.Text()) == ""
		}
	}
	if !countLine {
		t.Fatalf("empty current week should use a quiet spacer: %q", node.Text())
	}
}
