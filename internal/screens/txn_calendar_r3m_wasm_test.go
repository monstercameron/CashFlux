// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR3MCalendarMoneyUsesLedgerDisplayAndAccessibleDescriptions(t *testing.T) {
	f := render.New(t)
	f.Render(ui.Fragment(
		ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
			DateKey: "2026-10-03", Day: 3, InMonth: true, Count: 1,
			NetStr:    calNetStr(txncalendar.DayStat{Net: -675, Count: 1}, "USD"),
			NetTone:   calNetTone(txncalendar.DayStat{Net: -675, Count: 1}),
			AriaLabel: calDayAria(3, txncalendar.Cashflow{Spent: 675, Count: 1}, "USD"),
		}),
		ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
			DateKey: "2026-10-04", Day: 4, InMonth: true, Count: 1,
			NetStr:    calNetStr(txncalendar.DayStat{Net: 527075, Count: 1}, "USD"),
			NetTone:   calNetTone(txncalendar.DayStat{Net: 527075, Count: 1}),
			AriaLabel: calDayAria(4, txncalendar.Cashflow{Income: 527075, Count: 1}, "USD"),
		}),
		ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Cashflow: txncalendar.Cashflow{Spent: 675, Count: 1}, Base: "USD", WeekOf: "Oct 4", SummaryID: "week-one",
		}),
		ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Cashflow: txncalendar.Cashflow{Income: 527075, Count: 1}, Base: "USD", WeekOf: "Oct 11", SummaryID: "week-two",
		}),
	))

	buttons := f.AllByTag("button")
	if len(buttons) != 2 {
		t.Fatalf("calendar day buttons = %d, want 2", len(buttons))
	}
	if got := buttons[0].Text(); !strings.Contains(got, "−$6.75") || strings.Contains(got, "(") || strings.Contains(got, "-$") {
		t.Fatalf("expense calendar day text = %q, want −$6.75 with the typographic minus", got)
	}
	if got := buttons[0].Attr("aria-label"); !strings.Contains(got, "spent $6.75") {
		t.Fatalf("expense calendar day aria-label = %q, want spent $6.75", got)
	}
	if got := buttons[1].Text(); !strings.Contains(got, "+$5,270.75") {
		t.Fatalf("income calendar day text = %q, want +$5,270.75", got)
	}
	if got := buttons[1].Attr("aria-label"); !strings.Contains(got, "income $5,270.75") {
		t.Fatalf("income calendar day aria-label = %q, want income $5,270.75", got)
	}
	var netSpans []*render.QueryNode
	for _, node := range f.AllByTag("span") {
		if strings.Contains(node.Attr("class"), "txn-cal-net") {
			netSpans = append(netSpans, node)
		}
	}
	if len(netSpans) != 2 {
		t.Fatalf("rendered calendar day figures = %d, want 2", len(netSpans))
	}
	if got := netSpans[0].Attr("class"); !strings.Contains(got, "text-down") {
		t.Fatalf("expense day net class = %q, want negative tone", got)
	}
	if got := netSpans[1].Attr("class"); !strings.Contains(got, "text-up") {
		t.Fatalf("income day net class = %q, want text-up", got)
	}

	var totals []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == "txn-cal-week-total" {
			totals = append(totals, node)
		}
	}
	if len(totals) != 2 {
		t.Fatalf("calendar week totals = %d, want 2", len(totals))
	}
	if got := totals[0].Text(); !strings.Contains(got, "Spent −$6.75") {
		t.Fatalf("expense week total text = %q, want Spent −$6.75", got)
	}
	if got := totals[0].Attr("aria-label"); got != "" {
		t.Fatalf("week total aria-label = %q, want content-derived name", got)
	}
	if got := totals[0].Attr("aria-describedby"); got != "week-one" {
		t.Fatalf("expense week total aria-describedby = %q, want hidden summary id", got)
	}
	if got := totals[1].Text(); !strings.Contains(got, "Income $5,270.75") {
		t.Fatalf("income week total text = %q, want Income $5,270.75", got)
	}
	if got := totals[1].Attr("aria-describedby"); got != "week-two" {
		t.Fatalf("income week total aria-describedby = %q, want hidden summary id", got)
	}
}

func TestTQ3TransferDayShowsMovementWithoutCashSpending(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-12", Day: 12, InMonth: true, Count: 1,
		MovedStr: calMovedStr(50000, "USD"), AriaLabel: calDayAria(12, txncalendar.Cashflow{Moved: 50000, Count: 1}, "USD"),
	}))
	button := f.AllByTag("button")[0]
	if strings.Contains(button.Text(), "$0.00") || !strings.Contains(button.Text(), "Moved $500.00") {
		t.Fatalf("transfer-only day = %q, want movement and no cash total", button.Text())
	}
	if got := button.Attr("aria-label"); !strings.Contains(got, "spent $0.00") || !strings.Contains(got, "moved $500.00") {
		t.Fatalf("transfer day accessible name = %q, want zero spending and movement", got)
	}
}

func TestTQ3CalendarLabelsUseLocalizedCatalogHelpers(t *testing.T) {
	if got := calendarWeekday(2, false); got != "Tue" {
		t.Fatalf("short weekday = %q, want catalog value Tue", got)
	}
	if got := calendarWeekday(2, true); got != "Tuesday" {
		t.Fatalf("full weekday = %q, want catalog value Tuesday", got)
	}
	if got := calendarMonthYear(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); got != "October 2026" {
		t.Fatalf("month label = %q, want catalog month and year", got)
	}
}
