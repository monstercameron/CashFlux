// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestR1HFuturePeriodRendersScheduledGhostRows(t *testing.T) {
	f := render.New(t)
	due := time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC)
	f.Render(ui.CreateElement(txnNoMatch, txnNoMatchProps{
		FuturePeriod:   true,
		ScheduledItems: []bills.Bill{{Name: "Electric bill", Amount: money.New(8425, "USD"), DueDate: due}},
	}))

	var rows []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == "txn-scheduled-ghost" {
			rows = append(rows, node)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("scheduled ghost rows = %d, want 1", len(rows))
	}
	if got := rows[0].Text(); got == "" || !containsAll(got, "Electric bill", "$84.25", "Scheduled") {
		t.Fatalf("scheduled ghost row = %q, want date, payee, amount, and Scheduled status", got)
	}
}

func TestR1HTodayCalendarCellHasCurrentDateSemantics(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
		DateKey: "2026-10-18", Day: 18, InMonth: true, Today: true,
		NetStr: "$12.00", Count: 1,
	}))

	buttons := f.AllByTag("button")
	if len(buttons) != 1 {
		t.Fatalf("today cell buttons = %d, want 1", len(buttons))
	}
	if got := buttons[0].Attr("aria-current"); got != "date" {
		t.Fatalf("today cell aria-current = %q, want date", got)
	}
	if className := buttons[0].Attr("class"); !strings.Contains(className, "txn-cal-cell") || !strings.Contains(className, "today") {
		t.Fatalf("today cell class = %q, want token-styled today marker", className)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
