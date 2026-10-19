// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"
)

func TestFlexTargetForWindowScalesMonthlyTargetToCalendarWindow(t *testing.T) {
	tests := []struct {
		name       string
		start, end time.Time
		want       int64
	}{
		{
			name:  "week",
			start: time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, time.April, 13, 0, 0, 0, 0, time.UTC),
			want:  7000,
		},
		{
			name:  "quarter",
			start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			want:  90000,
		},
		{
			name:  "crosses month boundary",
			start: time.Date(2026, time.April, 29, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC),
			want:  4903,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := flexTargetForWindow(30000, tt.start, tt.end, "USD")
			if err != nil {
				t.Fatalf("flexTargetForWindow: %v", err)
			}
			if got.Amount != tt.want || got.Currency != "USD" {
				t.Fatalf("target = %v, want %d USD minor units", got, tt.want)
			}
		})
	}
}

func TestFlexTargetForWindowRejectsOverflow(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	if _, err := flexTargetForWindow(int64(^uint64(0)>>1), start, end, "USD"); err == nil {
		t.Fatal("flexTargetForWindow accepted a quarterly target that overflows int64")
	}
}

func TestFlexWallClockUsesLocalCalendarDate(t *testing.T) {
	local := time.Date(2026, time.November, 1, 0, 30, 0, 0, time.FixedZone("Pacific", -7*60*60))
	if got, want := budgetNowAt(local), time.Date(2026, time.November, 1, 0, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("budgetNowAt = %s, want wall-clock date %s", got, want)
	}
}
