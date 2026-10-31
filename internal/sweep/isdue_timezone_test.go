// SPDX-License-Identifier: MIT

package sweep

import (
	"testing"
	"time"
)

func TestIsDueCountsCalendarDaysInTheCallersZone(t *testing.T) {
	la := time.FixedZone("PDT", -7*3600)
	kiritimati := time.FixedZone("LINT", 14*3600)
	r := rule()
	r.LastProposed = "2026-03-01T00:00:00Z" // a stamp is the posted calendar day at midnight UTC
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"LA evening of day 29 is not due", time.Date(2026, 3, 30, 20, 0, 0, 0, la), false},
		{"LA morning of day 30 is due", time.Date(2026, 3, 31, 0, 30, 0, 0, la), true},
		{"Kiritimati morning of day 29 is not due", time.Date(2026, 3, 30, 1, 0, 0, 0, kiritimati), false},
		{"Kiritimati start of day 30 is due", time.Date(2026, 3, 31, 0, 30, 0, 0, kiritimati), true},
		{"UTC day 30 is due", time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), true},
		{"UTC end of day 29 is not due", time.Date(2026, 3, 30, 23, 59, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		if got := IsDue(r, tt.now); got != tt.want {
			t.Errorf("%s: IsDue = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestIsDueReadsAnOffsetStampAsItsOwnCalendarDay checks that a stamp carrying a UTC offset counts from
// the calendar day it shows, not from the UTC day. 2026-03-02T01:00+14:00 is Mar 1 in UTC but Mar 2
// on its own clock, so it falls due on Apr 1; 2026-03-01T23:00-07:00 is Mar 2 in UTC but Mar 1 on its
// own clock, so it falls due on Mar 31.
func TestIsDueReadsAnOffsetStampAsItsOwnCalendarDay(t *testing.T) {
	tests := []struct {
		name  string
		stamp string
		now   time.Time
		want  bool
	}{
		{"stamp ahead of UTC, last UTC instant before its day 30", "2026-03-02T01:00:00+14:00", time.Date(2026, 3, 31, 23, 59, 0, 0, time.UTC), false},
		{"stamp ahead of UTC, first UTC instant of its day 30", "2026-03-02T01:00:00+14:00", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), true},
		{"stamp behind UTC, last UTC instant before its day 30", "2026-03-01T23:00:00-07:00", time.Date(2026, 3, 30, 23, 59, 0, 0, time.UTC), false},
		{"stamp behind UTC, first UTC instant of its day 30", "2026-03-01T23:00:00-07:00", time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), true},
		{"stamp ahead of UTC, now ahead of UTC, day 29", "2026-03-02T01:00:00+14:00", time.Date(2026, 3, 31, 23, 0, 0, 0, time.FixedZone("LINT", 14*3600)), false},
		{"stamp ahead of UTC, now ahead of UTC, day 30", "2026-03-02T01:00:00+14:00", time.Date(2026, 4, 1, 0, 30, 0, 0, time.FixedZone("LINT", 14*3600)), true},
		{"stamp behind UTC, now behind UTC, day 29", "2026-03-01T23:00:00-07:00", time.Date(2026, 3, 30, 20, 0, 0, 0, time.FixedZone("PDT", -7*3600)), false},
		{"stamp behind UTC, now behind UTC, day 30", "2026-03-01T23:00:00-07:00", time.Date(2026, 3, 31, 0, 30, 0, 0, time.FixedZone("PDT", -7*3600)), true},
	}
	for _, tt := range tests {
		r := rule()
		r.LastProposed = tt.stamp
		if got := IsDue(r, tt.now); got != tt.want {
			t.Errorf("%s: IsDue = %v, want %v", tt.name, got, tt.want)
		}
	}
}
