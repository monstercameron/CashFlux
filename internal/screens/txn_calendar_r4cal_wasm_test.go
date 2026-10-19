// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR4CalendarScheduledItemsShowAmountsAndMissedState(t *testing.T) {
	f := render.New(t)
	f.Render(ui.Fragment(
		ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
			DateKey: "2026-10-03", Day: 3, InMonth: true, Count: 1,
			Rates:  currency.Rates{Base: "USD"},
			Ghosts: []txncalendar.Ghost{{Label: "Gym membership", Amount: -22000, Currency: "USD", NotPosted: true}},
		}),
		ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
			DateKey: "2026-10-22", Day: 22, InMonth: true,
			Rates:  currency.Rates{Base: "USD"},
			Ghosts: []txncalendar.Ghost{{Label: "Electric bill", Amount: -22000, Currency: "USD", Date: time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC)}},
		}),
	))

	buttons := f.AllByTag("button")
	if len(buttons) != 2 {
		t.Fatalf("calendar cells = %d, want 2", len(buttons))
	}
	for i, want := range []string{"not posted", "is due on this day"} {
		if got := buttons[i].Attr("aria-label"); !strings.Contains(got, want) {
			t.Errorf("day %d accessible name = %q, want scheduled item %q", i, got, want)
		}
	}
	if got := buttons[0].Text(); !strings.Contains(got, "$220.00 not posted") || !strings.Contains(got, "Gym membership") || strings.Contains(got, "·") {
		t.Errorf("missed scheduled item = %q, want amount and status on line one, name on line two, and no middle dot", got)
	}
	if got := buttons[1].Text(); !strings.Contains(got, "Due $220.00") || !strings.Contains(got, "Electric bill") || strings.Contains(got, "·") {
		t.Errorf("upcoming scheduled item = %q, want consistent Due label and no middle dot", got)
	}
	var missed *render.QueryNode
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-cal-ghost-missed") {
			missed = node
		}
	}
	if missed == nil || !strings.Contains(missed.Attr("aria-label"), "not posted") || !strings.Contains(missed.Attr("aria-label"), "$220.00") {
		t.Errorf("missed item's accessible description = %v, want status and amount", missed)
	}
}

func TestR4CalendarReportsUnavailableAmountWhenMissedGhostMagnitudeOverflows(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-03", Day: 3, InMonth: true,
		Rates:  currency.Rates{Base: "USD"},
		Ghosts: []txncalendar.Ghost{{Label: "Large bill", Amount: math.MinInt64, Currency: "USD", NotPosted: true}},
	}))
	buttons := f.AllByTag("button")
	if len(buttons) != 1 {
		t.Fatalf("calendar cells = %d, want 1", len(buttons))
	}
	want := uistate.T("transactions.calScheduledAmountUnavailable")
	if got := buttons[0].Attr("aria-label"); !strings.Contains(got, want) || !strings.Contains(got, "Large bill") {
		t.Errorf("day accessible name = %q, want unavailable amount and bill name", got)
	}
}

func TestR4CalendarDotLegendDescribesRenderedDensity(t *testing.T) {
	if got := uistate.T("transactions.calDotsLegend"); got != "More dots mean more transactions." {
		t.Fatalf("density legend = %q, want a description of increasing dot count", got)
	}
	for count, wantDots := range map[int]int{0: 0, 1: 1, 4: 1, 5: 1, 6: 2} {
		dots := calTransactionDotCount(count)
		if dots != wantDots {
			t.Errorf("count %d rendered %d dots, want %d", count, dots, wantDots)
		}
	}
}

func TestR4CalendarSearchHidesScheduledItems(t *testing.T) {
	ghosts := []txncalendar.Ghost{{RecurringID: "bill", Label: "Electric bill"}}
	if got := scheduledGhosts(ghosts, false); len(got) != 0 {
		t.Fatalf("scheduled items during text search = %d, want none", len(got))
	}
	if got := scheduledGhosts(ghosts, true); len(got) != 1 || got[0].RecurringID != "bill" {
		t.Fatalf("scheduled items without text search = %#v, want bill", got)
	}
}
