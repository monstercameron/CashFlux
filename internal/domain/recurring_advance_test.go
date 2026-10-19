// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"
)

func TestRecurringCadenceNextClampsShortMonthWithoutChangingDay(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		want  string
	}{
		{"29th to leap February", time.Date(2024, time.January, 29, 0, 0, 0, 0, time.UTC), "2024-02-29"},
		{"30th to February", time.Date(2026, time.January, 30, 0, 0, 0, 0, time.UTC), "2026-02-28"},
		{"31st to February", time.Date(2026, time.January, 31, 9, 30, 0, 0, time.UTC), "2026-02-28"},
		{"stored February 28 remains day 28", time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), "2026-03-28"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CadenceMonthly.Next(tt.start).Format("2006-01-02"); got != tt.want {
				t.Fatalf("Next = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRecurringCadenceNextKeepsLocalCalendarAndClock(t *testing.T) {
	loc := time.FixedZone("household", -5*60*60)
	start := time.Date(2026, time.January, 31, 21, 15, 17, 123, loc)
	want := time.Date(2026, time.February, 28, 21, 15, 17, 123, loc)
	if got := CadenceMonthly.Next(start); !got.Equal(want) || got.Location() != loc {
		t.Fatalf("Next = %s (%s), want %s (%s)", got, got.Location(), want, loc)
	}
}
