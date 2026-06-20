package payoff

import "testing"

func TestTrackProgress(t *testing.T) {
	tests := []struct {
		name              string
		baseline, current int64
		paid, rem         int64
		pct               int
	}{
		{"partway down", 100000, 60000, 40000, 60000, 40},
		{"fully paid off", 100000, 0, 100000, 0, 100},
		{"balance grew reads zero, never negative", 100000, 120000, 0, 120000, 0},
		{"no baseline, nothing owed is complete", 0, 0, 0, 0, 100},
		{"no baseline but still owing reads zero", 0, 5000, 0, 5000, 0},
		{"negative current is clamped", 50000, -10, 50000, 0, 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TrackProgress(tc.baseline, tc.current)
			if got.PaidOff != tc.paid || got.Remaining != tc.rem || got.Percent != tc.pct {
				t.Errorf("TrackProgress(%d,%d) = paid %d / rem %d / %d%%, want %d / %d / %d%%",
					tc.baseline, tc.current, got.PaidOff, got.Remaining, got.Percent, tc.paid, tc.rem, tc.pct)
			}
		})
	}
}
