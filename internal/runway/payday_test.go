// SPDX-License-Identifier: MIT

package runway

import (
	"testing"
	"time"
)

func TestNextPaydayHorizon(t *testing.T) {
	tests := []struct {
		name         string
		from         time.Time
		payCycleDay  int
		fallbackDays int
		want         int
	}{
		{
			name:         "day already passed this month — jumps to next month",
			from:         time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			payCycleDay:  15,
			fallbackDays: 14,
			want:         25, // Jun 20 → Jul 15 = 25 days
		},
		{
			name:         "day == today — minimum 1",
			from:         time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			payCycleDay:  15,
			fallbackDays: 14,
			want:         1, // would be 0, clamped to 1
		},
		{
			name:         "day later this month",
			from:         time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			payCycleDay:  15,
			fallbackDays: 14,
			want:         14, // Jun 1 → Jun 15 = 14 days
		},
		{
			name:         "day 31 in a 28-day month (February non-leap) — clamps to Feb 28",
			from:         time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			payCycleDay:  31,
			fallbackDays: 30,
			want:         27, // Feb 1 → Feb 28 = 27 days
		},
		{
			name:         "day 31 in a 30-day month (June) — clamps to Jun 30",
			from:         time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			payCycleDay:  31,
			fallbackDays: 30,
			want:         29, // Jun 1 → Jun 30 = 29 days
		},
		{
			name:         "payCycleDay == 0 — returns fallback",
			from:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			payCycleDay:  0,
			fallbackDays: 14,
			want:         14,
		},
		{
			name:         "payCycleDay < 0 — returns fallback",
			from:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			payCycleDay:  -5,
			fallbackDays: 30,
			want:         30,
		},
		{
			name:         "payCycleDay 31 passed in July (31-day month) — wraps to Aug 31",
			from:         time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			payCycleDay:  31,
			fallbackDays: 30,
			want:         30, // Aug 1 → Aug 31 = 30 days
		},
		{
			name:         "last day of month, target is next month",
			from:         time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			payCycleDay:  1,
			fallbackDays: 14,
			want:         1, // Jun 30 → Jul 1 = 1 day
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NextPaydayHorizon(tc.from, tc.payCycleDay, tc.fallbackDays)
			if got != tc.want {
				t.Errorf("NextPaydayHorizon(%v, %d, %d) = %d, want %d",
					tc.from.Format("2006-01-02"), tc.payCycleDay, tc.fallbackDays, got, tc.want)
			}
		})
	}
}
