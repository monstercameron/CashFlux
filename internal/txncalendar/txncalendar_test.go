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

func TestGhosts(t *testing.T) {
	rec := []domain.Recurring{
		{ID: "r1", Label: "Rent", Amount: usd(-120000), Cadence: domain.CadenceMonthly, NextDue: mustDate("2026-07-01")},
		{ID: "r2", Label: "Gym", Amount: usd(-5000), Cadence: domain.CadenceWeekly, NextDue: mustDate("2026-06-30")},
		{ID: "r3", Label: "Nope", Amount: usd(-100), Cadence: domain.CadenceMonthly},
	}
	got := Ghosts(rec, mustDate("2026-07-01"), mustDate("2026-08-01"))
	var rent, gym int
	for _, ghost := range got {
		switch ghost.RecurringID {
		case "r1":
			rent++
		case "r2":
			gym++
		case "r3":
			t.Error("zero-NextDue recurring must not project")
		}
	}
	if rent != 1 || gym != 4 {
		t.Fatalf("recurring occurrences = rent %d, gym %d; want 1 and 4", rent, gym)
	}
}
