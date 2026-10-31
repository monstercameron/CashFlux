// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"
	"time"
)

// TestLastMonthPresetsResolveFromMonthStart pins the rolling month presets on days that
// do not exist in the previous month. Shifting from the 31st by a month overflows into
// the current month, so "last month" must still name September on October 31.
func TestLastMonthPresetsResolveFromMonthStart(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		preset   string
		wantFrom string
		wantTo   string
	}{
		{"last month on Oct 31", time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC), PeriodLastMonth, "2026-09-01", "2026-09-30"},
		{"last month on Mar 31", time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC), PeriodLastMonth, "2026-02-01", "2026-02-28"},
		{"last month on May 31", time.Date(2026, time.May, 31, 12, 0, 0, 0, time.UTC), PeriodLastMonth, "2026-04-01", "2026-04-30"},
		{"three months on Aug 31", time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC), PeriodLastThreeMonths, "2026-06-01", "2026-08-31"},
		{"three months on Dec 31", time.Date(2026, time.December, 31, 12, 0, 0, 0, time.UTC), PeriodLastThreeMonths, "2026-10-01", "2026-12-31"},
		{"last month west of UTC", time.Date(2026, time.October, 31, 1, 0, 0, 0, time.FixedZone("west", -7*3600)), PeriodLastMonth, "2026-09-01", "2026-09-30"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Criteria{PeriodPreset: tc.preset}.ResolvePeriod(tc.now)
			if got.From != tc.wantFrom || got.To != tc.wantTo {
				t.Fatalf("ResolvePeriod(%s) = %q..%q, want %q..%q", tc.now.Format("2006-01-02"), got.From, got.To, tc.wantFrom, tc.wantTo)
			}
		})
	}
}

// TestInferLastMonthOnMonthEnd checks that a stored September range is still recognised
// as last month when the current date is October 31.
func TestInferLastMonthOnMonthEnd(t *testing.T) {
	now := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC)
	if got := InferPeriodPreset("2026-09-01", "2026-09-30", now); got != PeriodLastMonth {
		t.Fatalf("InferPeriodPreset(September) = %q, want %q", got, PeriodLastMonth)
	}
}
