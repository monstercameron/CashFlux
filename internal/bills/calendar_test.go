// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func billOn(id string, day time.Time) Bill {
	return Bill{AccountID: id, Name: id, Amount: money.New(1000, "USD"), DueDate: day}
}

func TestMonthCalendarShape(t *testing.T) {
	// June 2026, weeks starting Sunday.
	grid := MonthCalendar(nil, 2026, time.June, time.Sunday)
	if len(grid) == 0 {
		t.Fatal("empty grid")
	}
	for w, week := range grid {
		if len(week) != 7 {
			t.Fatalf("week %d has %d days, want 7", w, len(week))
		}
	}
	// First cell is the most recent Sunday on/before June 1.
	first := grid[0][0].Date
	if first.Weekday() != time.Sunday {
		t.Errorf("first cell weekday = %s, want Sunday", first.Weekday())
	}
	if first.After(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first cell %s is after June 1", first.Format("2006-01-02"))
	}
	// June 1 appears exactly once and is in-month.
	count := 0
	for _, week := range grid {
		for _, d := range week {
			if d.Date.Equal(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)) {
				count++
				if !d.InMonth {
					t.Error("June 1 marked out of month")
				}
			}
		}
	}
	if count != 1 {
		t.Errorf("June 1 appears %d times, want 1", count)
	}
}

func TestMonthCalendarWeekStartMonday(t *testing.T) {
	grid := MonthCalendar(nil, 2026, time.June, time.Monday)
	if grid[0][0].Date.Weekday() != time.Monday {
		t.Errorf("first cell weekday = %s, want Monday", grid[0][0].Date.Weekday())
	}
}

func TestMonthCalendarPlacesBills(t *testing.T) {
	due := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	outside := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC) // not in this grid
	grid := MonthCalendar([]Bill{billOn("visa", due), billOn("future", outside)}, 2026, time.June, time.Sunday)

	placed := 0
	for _, week := range grid {
		for _, d := range week {
			if len(d.Bills) > 0 {
				placed++
				if !d.Date.Equal(due) {
					t.Errorf("bill placed on %s, want June 15", d.Date.Format("2006-01-02"))
				}
				if d.Bills[0].AccountID != "visa" {
					t.Errorf("placed %s, want visa", d.Bills[0].AccountID)
				}
			}
			// Out-of-month cells never carry bills.
			if !d.InMonth && len(d.Bills) > 0 {
				t.Errorf("out-of-month cell %s carries bills", d.Date.Format("2006-01-02"))
			}
		}
	}
	if placed != 1 {
		t.Errorf("placed bills on %d days, want 1 (the August bill is outside the grid)", placed)
	}
}
