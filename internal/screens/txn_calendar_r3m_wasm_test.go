// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

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
			AriaLabel: calDayAria(3, txncalendar.DayStat{Net: -675, Count: 1}, "USD"),
		}),
		ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
			DateKey: "2026-10-04", Day: 4, InMonth: true, Count: 1,
			NetStr:    calNetStr(txncalendar.DayStat{Net: 527075, Count: 1}, "USD"),
			NetTone:   calNetTone(txncalendar.DayStat{Net: 527075, Count: 1}),
			AriaLabel: calDayAria(4, txncalendar.DayStat{Net: 527075, Count: 1}, "USD"),
		}),
		ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Stat: txncalendar.DayStat{Net: -675, Count: 1}, Base: "USD", Spent: 675, WeekOf: "Oct 4",
		}),
		ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Stat: txncalendar.DayStat{Net: 527075, Count: 1}, Base: "USD", Income: 527075, WeekOf: "Oct 11",
		}),
	))

	buttons := f.AllByTag("button")
	if len(buttons) != 2 {
		t.Fatalf("calendar day buttons = %d, want 2", len(buttons))
	}
	if got := buttons[0].Text(); !strings.Contains(got, "$6.75") || strings.Contains(got, "(") || strings.Contains(got, "-$") {
		t.Fatalf("expense calendar day text = %q, want neutral $6.75 without parentheses or minus", got)
	}
	if got := buttons[0].Attr("aria-label"); !strings.Contains(got, "spent $6.75") {
		t.Fatalf("expense calendar day aria-label = %q, want spent $6.75", got)
	}
	if got := buttons[1].Text(); !strings.Contains(got, "+$5,270.75") {
		t.Fatalf("income calendar day text = %q, want +$5,270.75", got)
	}
	if got := buttons[1].Attr("aria-label"); !strings.Contains(got, "net +$5,270.75") {
		t.Fatalf("income calendar day aria-label = %q, want net +$5,270.75", got)
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
	if got := netSpans[0].Attr("class"); !strings.Contains(got, "txn-amount-expense") || strings.Contains(got, "text-down") {
		t.Fatalf("expense day net class = %q, want neutral expense style", got)
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
	if got := totals[0].Text(); !strings.Contains(got, "Spent $6.75") {
		t.Fatalf("expense week total text = %q, want spent $6.75", got)
	}
	if got := totals[0].Attr("aria-label"); !strings.Contains(got, "spent $6.75") || !strings.Contains(got, "1 transaction") {
		t.Fatalf("expense week total aria-label = %q, want spent $6.75 and one transaction", got)
	}
	if got := totals[1].Text(); !strings.Contains(got, "+$5,270.75 in") {
		t.Fatalf("income week total text = %q, want +$5,270.75 in", got)
	}
	if got := totals[1].Attr("aria-label"); !strings.Contains(got, "+$5,270.75 in") || !strings.Contains(got, "1 transaction") {
		t.Fatalf("income week total aria-label = %q, want +$5,270.75 in and one transaction", got)
	}
}
