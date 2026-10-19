// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFoldSettledScheduledItems(t *testing.T) {
	anchor := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return time.Date(2026, time.October, n, 0, 0, 0, 0, time.UTC) }
	recurring := []domain.Recurring{
		{ID: "late", Label: "Gym membership", Amount: money.New(-22000, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(3)},
		{ID: "paid", Label: "Phone bill", Amount: money.New(-5000, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(5)},
		{ID: "future", Label: "Electric bill", Amount: money.New(-8000, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(22)},
	}
	month, err := MonthCashflow(anchor, time.Sunday, nil, recurring, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	settled := map[string]bool{domain.OccurrenceKey("paid", day(5)): true}
	got := FoldSettled(month.Weeks, settled, day(19))

	items := map[string]Ghost{}
	for _, week := range got {
		for _, cell := range week.Days {
			for _, ghost := range cell.Ghosts {
				items[ghost.RecurringID] = ghost
			}
		}
	}
	if _, ok := items["paid"]; ok {
		t.Fatal("settled occurrence remained scheduled")
	}
	if late := items["late"]; !late.NotPosted || late.Amount != -22000 || late.Currency != "USD" {
		t.Errorf("overdue item = %+v, want not-posted USD -22000", late)
	}
	if future := items["future"]; future.NotPosted || future.Amount != -8000 {
		t.Errorf("upcoming item = %+v, want scheduled USD -8000", future)
	}
	// The fold returns independent slices so a display projection cannot mutate
	// the original month or another consumer's view.
	if len(month.Weeks) == 0 || len(month.Weeks[0].Days) == 0 {
		t.Fatal("source month unexpectedly empty")
	}
	if len(month.Weeks[0].Days[0].Ghosts) > 0 && month.Weeks[0].Days[0].Ghosts[0].NotPosted {
		t.Fatal("fold mutated the source projection")
	}
}
