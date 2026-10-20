// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txncalendarview"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCalendarBusyDayPreviewsUrgentDueAndRetainsAllDetails(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-05", Day: 5, InMonth: true,
		Rates: currency.Rates{Base: "USD"},
		Ghosts: []txncalendar.Ghost{
			{Label: "Streaming", Amount: -3800, Currency: "USD"},
			{Label: "Software", Amount: -3900, Currency: "USD"},
			{Label: "Rent", Amount: -120000, Currency: "USD", NotPosted: true},
		},
	}))
	button := f.AllByTag("button")[0]
	if text := button.Text(); !strings.Contains(text, "Rent") || !strings.Contains(text, "+2 more due") || strings.Contains(text, "Streaming") || strings.Contains(text, "Software") {
		t.Errorf("busy day preview = %q", text)
	}
	for _, name := range []string{"Streaming", "Software", "Rent"} {
		if !strings.Contains(button.Attr("aria-label"), name) {
			t.Errorf("%s missing from complete day description", name)
		}
	}
	if got := button.Attr("title"); got != "" {
		t.Errorf("day button duplicates its accessible name as a tooltip: %q", got)
	}
}

func TestCalendarOverflowDisclosureListsHiddenDueItems(t *testing.T) {
	f := render.New(t)
	f.Render(txnCalOverflow([]txncalendarview.Week{{Days: []txncalendarview.Day{{
		Day: 5, InMonth: true, Rates: currency.Rates{Base: "USD"},
		Ghosts: []txncalendar.Ghost{
			{Label: "Streaming", Amount: -3800, Currency: "USD"},
			{Label: "Software", Amount: -3900, Currency: "USD"},
			{Label: "Rent", Amount: -120000, Currency: "USD", NotPosted: true},
		},
	}}}}))
	summaries := f.AllByTag("summary")
	if len(summaries) != 1 || summaries[0].Text() != "Other due items (2)" {
		t.Fatalf("overflow summary = %#v", summaries)
	}
	details := f.AllByTag("details")
	if len(details) != 1 {
		t.Fatalf("overflow disclosures = %d, want 1", len(details))
	}
	text := details[0].Text()
	if !strings.Contains(text, "Day 5") || !strings.Contains(text, "Streaming") || !strings.Contains(text, "Software") || strings.Contains(text, "Rent") {
		t.Errorf("overflow details = %q", text)
	}
}
