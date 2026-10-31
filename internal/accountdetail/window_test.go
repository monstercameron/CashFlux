// SPDX-License-Identifier: MIT

package accountdetail

import (
	"testing"
	"time"
)

func TestSelectWindow(t *testing.T) {
	asOf := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	yearAndAHalf := asOf.AddDate(0, -18, 0)
	tests := []struct {
		name     string
		selected string
		earliest time.Time
		want     Window
	}{
		{"no history keeps 90 days", Range90d, time.Time{}, Window{Days: 90}},
		{"no history ignores all", RangeAll, time.Time{}, Window{Days: 90}},
		{"short history ignores 12m", Range12m, asOf.AddDate(0, 0, -30), Window{Days: 90}},
		{"long history 90d", Range90d, yearAndAHalf, Window{Days: 90, HasRange: true}},
		{"long history 12m", Range12m, yearAndAHalf, Window{Days: 365, HasRange: true}},
		{"long history all spans back to first transaction", RangeAll, yearAndAHalf, Window{Days: 547, HasRange: true}},
		{"unknown key selects 90 days", "weekly", yearAndAHalf, Window{Days: 90, HasRange: true}},
		{"all is capped at twenty years", RangeAll, asOf.AddDate(-60, 0, 0), Window{Days: 366 * 20, HasRange: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectWindow(tc.selected, asOf, tc.earliest); got != tc.want {
				t.Fatalf("SelectWindow(%q) = %+v, want %+v", tc.selected, got, tc.want)
			}
		})
	}
}

func TestSelectWindowCountsCalendarDaysInTheClockLocation(t *testing.T) {
	west := time.FixedZone("UTC-8", -8*60*60)
	east := time.FixedZone("UTC+9", 9*60*60)
	tests := []struct {
		name     string
		asOf     time.Time
		earliest time.Time
		want     Window
	}{
		{
			// Jul 19 to Apr 21 is 89 days: a 90-day span, so no range picker.
			"evening in a negative offset",
			time.Date(2026, time.July, 19, 20, 0, 0, 0, west),
			time.Date(2026, time.April, 21, 0, 0, 0, 0, time.UTC),
			Window{Days: 90},
		},
		{
			"one more calendar day in a negative offset offers the picker",
			time.Date(2026, time.July, 19, 20, 0, 0, 0, west),
			time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC),
			Window{Days: 91, HasRange: true},
		},
		{
			"early morning in a positive offset",
			time.Date(2026, time.July, 19, 1, 0, 0, 0, east),
			time.Date(2026, time.April, 21, 23, 0, 0, 0, east),
			Window{Days: 90},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectWindow(RangeAll, tc.asOf, tc.earliest); got != tc.want {
				t.Fatalf("SelectWindow = %+v, want %+v", got, tc.want)
			}
		})
	}
}
