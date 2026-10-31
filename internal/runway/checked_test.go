// SPDX-License-Identifier: MIT

package runway

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestProjectCheckedMatchesProject(t *testing.T) {
	from := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		rec("rent", -10000, "USD", domain.CadenceMonthly, from.AddDate(0, 0, 4)),
		rec("pay", 30000, "EUR", domain.CadenceWeekly, from.AddDate(0, 0, 1)),
	}
	want, err := Project(20000, recs, from, 30, 5000, usd())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProjectChecked(20000, recs, from, 30, 5000, usd())
	if err != nil || got.MinBalance != want.MinBalance || got.BreachDay != want.BreachDay || got.BreachShortfall != want.BreachShortfall || len(got.Daily) != len(want.Daily) {
		t.Fatalf("checked=%+v err=%v, want %+v", got, err, want)
	}
	for i := range got.Daily {
		if got.Daily[i] != want.Daily[i] {
			t.Fatalf("day %d=%+v, want %+v", i, got.Daily[i], want.Daily[i])
		}
	}
}

func TestProjectCheckedReportsFXOverflowAndHorizonAvailability(t *testing.T) {
	from := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		recs  []domain.Recurring
		days  int
		rates currency.Rates
	}{
		{name: "missing FX", recs: []domain.Recurring{rec("rent", -1, "EUR", domain.CadenceMonthly, from)}, days: 10, rates: currency.Rates{Base: "USD"}},
		{name: "event overflow", recs: []domain.Recurring{rec("large", math.MaxInt64, "USD", domain.CadenceDaily, from), rec("extra", 1, "USD", domain.CadenceDaily, from)}, days: 1, rates: currency.Rates{Base: "USD"}},
		{name: "horizon bound", days: cashflow.MaxCheckedProjectionDays + 1, rates: currency.Rates{Base: "USD"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projection, err := ProjectChecked(0, tc.recs, from, tc.days, 0, tc.rates)
			if err == nil || len(projection.Daily) != 0 {
				t.Fatalf("invalid projection=%+v err=%v", projection, err)
			}
			if tc.name == "event overflow" && !strings.Contains(err.Error(), "overflow") {
				t.Fatalf("event overflow error = %v", err)
			}
		})
	}
}

func TestEventsCheckedBoundsLargeSchedule(t *testing.T) {
	from := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	recs := make([]domain.Recurring, 28)
	for i := range recs {
		recs[i] = rec("daily", 1, "USD", domain.CadenceDaily, from)
	}
	if events, err := EventsChecked(recs, from, cashflow.MaxCheckedProjectionDays, currency.Rates{Base: "USD"}); err == nil || events != nil {
		t.Fatalf("unbounded events returned %d events, %v", len(events), err)
	}
}
