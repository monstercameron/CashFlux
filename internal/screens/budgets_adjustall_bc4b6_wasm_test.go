// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"
)

func TestBC4B6AdjustAllUsesWallCalendarDateAtPeriodBoundary(t *testing.T) {
	end := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{
			name: "Pacific local October 31 remains live",
			now:  time.Date(2026, time.October, 31, 18, 0, 0, 0, time.FixedZone("PDT", -7*60*60)),
			want: false,
		},
		{
			name: "Tokyo local November 1 is beyond October",
			now:  time.Date(2026, time.November, 1, 5, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adjustAllPeriodClosed(end, tc.now); got != tc.want {
				t.Errorf("adjustAllPeriodClosed() = %t, want %t", got, tc.want)
			}
		})
	}
}
