// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func usd(n int64) money.Money { return money.New(n, "USD") }

func TestBucketByDay(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "a", Amount: usd(-1000), Date: mustDate("2026-07-03")},
		{ID: "b", Amount: usd(-500), Date: mustDate("2026-07-03")},
		{ID: "c", Amount: usd(2000), Date: mustDate("2026-07-05")},
	}
	got := BucketByDay(txns)
	if got["2026-07-03"].Net != -1500 || got["2026-07-03"].Count != 2 {
		t.Errorf("day 03 = %+v, want net -1500 count 2", got["2026-07-03"])
	}
	if got["2026-07-05"].Net != 2000 || got["2026-07-05"].Count != 1 {
		t.Errorf("day 05 = %+v, want net 2000 count 1", got["2026-07-05"])
	}
	if _, ok := got["2026-07-04"]; ok {
		t.Error("empty day should be absent")
	}
}

func TestGhosts(t *testing.T) {
	rec := []domain.Recurring{
		{ID: "r1", Label: "Rent", Amount: usd(-120000), Cadence: domain.CadenceMonthly, NextDue: mustDate("2026-07-01")},
		{ID: "r2", Label: "Gym", Amount: usd(-5000), Cadence: domain.CadenceWeekly, NextDue: mustDate("2026-06-30")},
		{ID: "r3", Label: "Nope", Amount: usd(-100), Cadence: domain.CadenceMonthly}, // zero NextDue → skipped
	}
	from := mustDate("2026-07-01")
	to := mustDate("2026-08-01")
	got := Ghosts(rec, from, to)

	var rent, gym int
	for _, g := range got {
		switch g.RecurringID {
		case "r1":
			rent++
		case "r2":
			gym++
		case "r3":
			t.Error("zero-NextDue recurring must not project")
		}
	}
	if rent != 1 {
		t.Errorf("rent occurrences = %d, want 1", rent)
	}
	// Gym: weekly from Jun 30 → Jul 7,14,21,28 all within July (Jun 30 is before window).
	if gym != 4 {
		t.Errorf("gym occurrences = %d, want 4", gym)
	}
}

func TestMonthGrid(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "a", Amount: usd(-1000), Date: mustDate("2026-07-15")},
	}
	rec := []domain.Recurring{
		{ID: "r1", Label: "Rent", Amount: usd(-120000), Cadence: domain.CadenceMonthly, NextDue: mustDate("2026-07-01")},
	}
	weeks := Month(mustDate("2026-07-10"), time.Sunday, txns, rec)

	// July 2026 spans 5 weeks when the week starts Sunday (Jul 1 is a Wednesday).
	if len(weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(weeks))
	}
	for _, w := range weeks {
		if len(w) != 7 {
			t.Fatalf("week length = %d, want 7", len(w))
		}
	}
	// First cell must be a Sunday.
	if weeks[0][0].Date.Weekday() != time.Sunday {
		t.Errorf("grid start weekday = %v, want Sunday", weeks[0][0].Date.Weekday())
	}

	// Find the July 15 cell: net -1000, in month.
	var found bool
	for _, w := range weeks {
		for _, c := range w {
			if DayKey(c.Date) == "2026-07-15" {
				found = true
				if !c.InMonth || c.Stat.Net != -1000 {
					t.Errorf("Jul 15 cell = %+v, want inMonth net -1000", c)
				}
			}
			if DayKey(c.Date) == "2026-07-01" && len(c.Ghosts) != 1 {
				t.Errorf("Jul 1 ghosts = %d, want 1 (rent)", len(c.Ghosts))
			}
		}
	}
	if !found {
		t.Error("Jul 15 cell missing from grid")
	}

	// Padding days (June/August) must carry no stat.
	for _, c := range weeks[0] {
		if !c.InMonth && (c.Stat.Count != 0 || len(c.Ghosts) != 0) {
			t.Errorf("padding cell %s has data", DayKey(c.Date))
		}
	}
}

func TestMonthGridMondayStart(t *testing.T) {
	weeks := Month(mustDate("2026-07-10"), time.Monday, nil, nil)
	if weeks[0][0].Date.Weekday() != time.Monday {
		t.Errorf("grid start weekday = %v, want Monday", weeks[0][0].Date.Weekday())
	}
}
