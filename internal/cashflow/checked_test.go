// SPDX-License-Identifier: MIT

package cashflow

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestDailyBalancesCheckedMatchesCanonicalProjection(t *testing.T) {
	events := []Event{{Day: 0, Amount: 10}, {Day: 1, Amount: -25}, {Day: 1, Amount: 3}, {Day: 4, Amount: 500}, {Day: -1, Amount: 900}}
	want := DailyBalances(100, events, 3, 90)
	got, err := DailyBalancesChecked(100, events, 3, 90)
	if err != nil || len(got.Daily) != len(want.Daily) || got.MinBalance != want.MinBalance || got.MinDay != want.MinDay || got.BreachDay != want.BreachDay || got.BreachShortfall != want.BreachShortfall {
		t.Fatalf("checked projection=%+v err=%v; want %+v", got, err, want)
	}
	for i := range got.Daily {
		if got.Daily[i] != want.Daily[i] {
			t.Fatalf("day %d=%+v, want %+v", i, got.Daily[i], want.Daily[i])
		}
	}
}

func TestDailyBalancesCheckedRejectsMoneyAndWorkOverflow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		start  int64
		events []Event
		buffer int64
		days   int
	}{
		{name: "event sum", events: []Event{{Day: 0, Amount: math.MaxInt64}, {Day: 0, Amount: 1}}, days: 1},
		{name: "daily balance", start: math.MaxInt64, events: []Event{{Day: 0, Amount: 1}}, days: 1},
		{name: "breach shortfall", start: math.MinInt64, buffer: math.MaxInt64, days: 1},
		{name: "days bound", days: MaxCheckedProjectionDays + 1},
		{name: "events bound", events: make([]Event, MaxCheckedProjectionEvents+1), days: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DailyBalancesChecked(tc.start, tc.events, tc.days, tc.buffer)
			if err == nil {
				t.Fatalf("projection unexpectedly succeeded: %+v", got)
			}
			if tc.name != "days bound" && tc.name != "events bound" && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("error = %v, want money overflow", err)
			}
			if len(got.Daily) != 0 {
				t.Fatalf("partial projection returned with error: %+v", got)
			}
		})
	}
}
