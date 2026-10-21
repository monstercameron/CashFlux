// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFlexWindowTargetPreservesCalendarProrationAndSegmentRounding(t *testing.T) {
	for _, tc := range []struct {
		name       string
		monthly    int64
		start, end time.Time
		want       int64
	}{
		{"week", 30000, flexDate(2026, 4, 6), flexDate(2026, 4, 13), 7000},
		{"quarter", 30000, flexDate(2026, 4, 1), flexDate(2026, 7, 1), 90000},
		{"partial months", 30000, flexDate(2026, 4, 29), flexDate(2026, 5, 4), 4903},
		{"leap month", 29000, flexDate(2024, 2, 1), flexDate(2024, 3, 1), 29000},
		{"per-segment rounding stays independent", 1, flexDate(2026, 5, 31), flexDate(2026, 6, 2), 0},
		{"half away positive", 15, flexDate(2026, 4, 1), flexDate(2026, 4, 2), 1},
		{"half away negative", -15, flexDate(2026, 4, 1), flexDate(2026, 4, 2), -1},
		{"empty", 30000, flexDate(2026, 4, 1), flexDate(2026, 4, 1), 0},
		{"reversed", 30000, flexDate(2026, 4, 2), flexDate(2026, 4, 1), 0},
		{"full extreme monthly amount", math.MaxInt64, flexDate(2026, 4, 1), flexDate(2026, 5, 1), math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FlexTargetForWindow(tc.monthly, tc.start, tc.end, "USD")
			if err != nil || got != money.New(tc.want, "USD") {
				t.Fatalf("got %v %v; want%d USD", got, err, tc.want)
			}
		})
	}
}

func TestFlexWindowTargetUsesWallDatesAcrossTimezonesAndRejectsAggregateOverflow(t *testing.T) {
	start := time.Date(2026, 3, 8, 23, 30, 0, 0, time.FixedZone("west", -5*3600))
	end := time.Date(2026, 3, 15, 1, 30, 0, 0, time.FixedZone("east", -4*3600))
	got, err := FlexTargetForWindow(31000, start, end, "EUR")
	if err != nil || got != money.New(7000, "EUR") {
		t.Fatalf("elapsed hours changed calendar target: %v %v", got, err)
	}
	got, err = FlexTargetForWindow(math.MaxInt64, flexDate(2026, 4, 1), flexDate(2026, 7, 1), "USD")
	if !errors.Is(err, money.ErrOverflow) || got.Amount != 0 {
		t.Fatalf("aggregate overflow became a valid target: %v %v", got, err)
	}
}
