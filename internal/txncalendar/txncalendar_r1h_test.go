// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestWeekStatTotalsDailyNetAndCountsIncludingPadding(t *testing.T) {
	week := []Cell{
		{Date: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), InMonth: false},
		{Date: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), InMonth: true, Stat: DayStat{Net: -1200, Count: 2}},
		{Date: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), InMonth: true, Stat: DayStat{Net: 3200, Count: 1}},
		{Date: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), InMonth: true, Stat: DayStat{Net: -500, Count: 1}},
	}
	got := WeekStat(week)
	if got != (DayStat{Net: 1500, Count: 4}) {
		t.Fatalf("WeekStat() = %+v, want net 1500 and count 4", got)
	}
}

func TestWeekStatEmptyWeekIsZero(t *testing.T) {
	if got := WeekStat(make([]Cell, 7)); got != (DayStat{}) {
		t.Fatalf("WeekStat(empty week) = %+v, want zero", got)
	}
}

func TestWeekStatMatchesMonthProjection(t *testing.T) {
	weeks := Month(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Sunday,
		[]domain.Transaction{{Date: time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), Amount: money.New(-700, "USD")},
			{Date: time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), Amount: money.New(1000, "USD")}}, nil)
	for _, week := range weeks {
		stat := WeekStat(week)
		for _, cell := range week {
			if cell.Date.Format("2006-01-02") == "2026-10-03" && (stat != (DayStat{Net: 300, Count: 2})) {
				t.Fatalf("week containing October 3 totals %+v, want net 300 and count 2", stat)
			}
		}
	}
}
