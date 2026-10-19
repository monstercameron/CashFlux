// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCalTC4N3ScheduledDetailsUseTheDayButtonAccessibleName(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-22", Day: 22, InMonth: true, AriaLabel: "October 22",
		Rates: currency.Rates{Base: "USD"},
		Ghosts: []txncalendar.Ghost{{Label: "Rewards Credit Card", Amount: -22000, Currency: "USD",
			Date: time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC), Liability: true}},
	}))
	button := f.AllByTag("button")[0]
	if got := button.Attr("aria-label"); !strings.Contains(got, "Rewards Credit Card is due on this day for $220.00") {
		t.Fatalf("day accessible name = %q, want its scheduled liability details", got)
	}
	if got := button.Attr("title"); got != "" {
		t.Fatalf("day button duplicates its accessible name as a tooltip: %q", got)
	}
	for _, span := range f.AllByTag("span") {
		if span.Attr("aria-label") != "" || span.Attr("title") != "" {
			t.Fatalf("non-interactive schedule span owns a redundant label or tooltip: %+v", span)
		}
	}
}

func TestCalTC4N3DotsUseInlineMarkupInsideButton(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{DateKey: "2026-10-22", Day: 22, InMonth: true, Count: 6}))
	for _, div := range f.AllByTag("div") {
		if strings.Contains(div.Attr("class"), "txn-cal-dots") {
			t.Fatal("transaction dots use a block div inside the day button")
		}
	}
	var found bool
	for _, span := range f.AllByTag("span") {
		if strings.Contains(span.Attr("class"), "txn-cal-dots") {
			found = true
		}
	}
	if !found {
		t.Fatal("transaction dot wrapper is missing its inline span")
	}
}

func TestCalTC4N3EmptyWeekSummaryRemainsAccessible(t *testing.T) {
	f := render.New(t)
	f.Render(txnCalWeekTotal(txnCalWeekTotalProps{Base: "USD", WeekOf: "Oct 4", SummaryID: "week", Future: true}))
	var found bool
	for _, span := range f.AllByTag("span") {
		if !strings.Contains(span.Attr("class"), "txn-cal-empty-week") {
			continue
		}
		found = true
		if span.Attr("aria-hidden") == "true" {
			t.Fatal("week's zero-spend explanation is hidden from assistive technology")
		}
		if strings.TrimSpace(span.Text()) == "" {
			t.Fatal("week's accessible zero-spend explanation is empty")
		}
	}
	if !found {
		t.Fatal("empty future week has no screen-reader zero-spend explanation")
	}
}
