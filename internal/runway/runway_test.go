// SPDX-License-Identifier: MIT

package runway

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd() currency.Rates { return currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}} }

func rec(label string, minor int64, code string, cad domain.RecurringCadence, due time.Time) domain.Recurring {
	return domain.Recurring{Label: label, Amount: money.New(minor, code), Cadence: cad, NextDue: due}
}

func TestEventsOffsetsSignsAndCadence(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		rec("rent", -100000, "USD", domain.CadenceMonthly, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)),
		rec("pay", 50000, "USD", domain.CadenceWeekly, time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)),
	}
	ev, err := Events(recs, from, 30, usd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// rent: once (Jun 10 = day 9). pay: Jun 3,10,17,24 = days 2,9,16,23.
	if len(ev) != 5 {
		t.Fatalf("got %d events, want 5: %+v", len(ev), ev)
	}
	var rentDays, payDays []int
	for _, e := range ev {
		switch e.Label {
		case "rent":
			rentDays = append(rentDays, e.Day)
			if e.Amount != -100000 {
				t.Errorf("rent amount = %d, want -100000", e.Amount)
			}
		case "pay":
			payDays = append(payDays, e.Day)
			if e.Amount != 50000 {
				t.Errorf("pay amount = %d, want 50000", e.Amount)
			}
		}
	}
	if len(rentDays) != 1 || rentDays[0] != 9 {
		t.Errorf("rent days = %v, want [9]", rentDays)
	}
	if !equalInts(payDays, []int{2, 9, 16, 23}) {
		t.Errorf("pay days = %v, want [2 9 16 23]", payDays)
	}
}

func TestEventsFastForwardsStaleSchedule(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	// NextDue is a week before `from`; weekly → first in-range occurrence is Jun 1.
	recs := []domain.Recurring{rec("gym", -5000, "USD", domain.CadenceWeekly, time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC))}
	ev, err := Events(recs, from, 10, usd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Jun 1 (day 0), Jun 8 (day 7); Jun 15 is out of the 10-day horizon.
	days := []int{}
	for _, e := range ev {
		days = append(days, e.Day)
		if e.Amount != -5000 {
			t.Errorf("amount = %d, want -5000", e.Amount)
		}
	}
	if !equalInts(days, []int{0, 7}) {
		t.Errorf("days = %v, want [0 7]", days)
	}
}

func TestEventsConvertsToBase(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	// -100.00 EUR at 1 EUR = 1.10 USD → -110.00 USD = -11000 minor.
	recs := []domain.Recurring{rec("eu sub", -10000, "EUR", domain.CadenceMonthly, time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC))}
	ev, err := Events(recs, from, 30, usd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ev) != 1 || ev[0].Amount != -11000 {
		t.Fatalf("got %+v, want one event of -11000 (base USD)", ev)
	}
}

func TestProjectFlagsBreach(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	// Start with $200; a $300 bill on day 5 drops the balance below a $0 buffer.
	recs := []domain.Recurring{rec("bill", -30000, "USD", domain.CadenceYearly, time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC))}
	p, err := Project(20000, recs, from, 30, 0, usd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.WillBreach() || p.BreachDay != 5 {
		t.Errorf("BreachDay = %d (breach=%v), want day 5", p.BreachDay, p.WillBreach())
	}
	if p.BreachShortfall != 10000 {
		t.Errorf("BreachShortfall = %d, want 10000", p.BreachShortfall)
	}
}

func TestEventsEmptyHorizon(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{rec("x", -100, "USD", domain.CadenceWeekly, from)}
	if ev, err := Events(recs, from, 0, usd()); err != nil || ev != nil {
		t.Errorf("empty horizon should yield no events, got %+v (err %v)", ev, err)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
