// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"testing"
	"time"
)

// series builds a date slice by stepping from start with a fixed function.
func stepSeries(start time.Time, n int, step func(time.Time) time.Time) []time.Time {
	out := make([]time.Time, 0, n)
	cur := start
	for i := 0; i < n; i++ {
		out = append(out, cur)
		cur = step(cur)
	}
	return out
}

func TestDetectRhythmCadences(t *testing.T) {
	tests := []struct {
		name  string
		dates []time.Time
		want  Cadence
	}{
		{
			name:  "weekly",
			dates: stepSeries(d(2026, 1, 5), 5, func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }),
			want:  CadenceWeekly,
		},
		{
			name:  "biweekly constant 14",
			dates: stepSeries(d(2026, 1, 5), 7, func(t time.Time) time.Time { return t.AddDate(0, 0, 14) }),
			want:  CadenceBiweekly,
		},
		{
			name: "semi-monthly 1st and 15th",
			dates: []time.Time{
				d(2026, 1, 1), d(2026, 1, 15), d(2026, 2, 1), d(2026, 2, 15),
				d(2026, 3, 1), d(2026, 3, 15), d(2026, 4, 1),
			},
			want: CadenceSemimonthly,
		},
		{
			name:  "every 4 weeks (28-day, DOM drifts)",
			dates: stepSeries(d(2026, 1, 5), 6, func(t time.Time) time.Time { return t.AddDate(0, 0, 28) }),
			want:  CadenceEvery4Weeks,
		},
		{
			name:  "monthly by DOM",
			dates: stepSeries(d(2026, 1, 9), 6, func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }),
			want:  CadenceMonthly,
		},
		{
			name:  "quarterly",
			dates: stepSeries(d(2025, 1, 15), 5, func(t time.Time) time.Time { return t.AddDate(0, 3, 0) }),
			want:  CadenceQuarterly,
		},
		{
			name:  "semiannual",
			dates: stepSeries(d(2024, 1, 15), 4, func(t time.Time) time.Time { return t.AddDate(0, 6, 0) }),
			want:  CadenceSemiannual,
		},
		{
			name:  "annual",
			dates: stepSeries(d(2022, 1, 15), 4, func(t time.Time) time.Time { return t.AddDate(1, 0, 0) }),
			want:  CadenceAnnual,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectRhythm(tt.dates)
			if got.cadence != tt.want {
				t.Errorf("cadence = %v, want %v (fit %.2f)", got.cadence, tt.want, got.fit)
			}
			if got.fit < minRhythmFit {
				t.Errorf("fit %.2f below usable floor for a clean series", got.fit)
			}
		})
	}
}

// TestBiweeklyNotSemimonthly guards the specific look-alike: a constant 14-day
// gap whose anchor walks the month is biweekly, not semi-monthly.
func TestBiweeklyNotSemimonthly(t *testing.T) {
	dates := stepSeries(d(2026, 1, 2), 8, func(t time.Time) time.Time { return t.AddDate(0, 0, 14) })
	if got := detectRhythm(dates); got.cadence != CadenceBiweekly {
		t.Errorf("got %v, want biweekly", got.cadence)
	}
}

// TestFourWeeklyNotMonthly guards the every-4-weeks vs monthly look-alike.
func TestFourWeeklyNotMonthly(t *testing.T) {
	four := stepSeries(d(2026, 1, 6), 8, func(t time.Time) time.Time { return t.AddDate(0, 0, 28) })
	if got := detectRhythm(four); got.cadence != CadenceEvery4Weeks {
		t.Errorf("28-day series = %v, want every-4-weeks", got.cadence)
	}
	monthly := stepSeries(d(2026, 1, 20), 8, func(t time.Time) time.Time { return t.AddDate(0, 1, 0) })
	if got := detectRhythm(monthly); got.cadence != CadenceMonthly {
		t.Errorf("monthly series = %v, want monthly", got.cadence)
	}
}

// TestAnchorWindow checks the anchor day + posting window inference for a monthly
// bill that posts around the 9th but sometimes lands on the 11th.
func TestAnchorWindow(t *testing.T) {
	dates := []time.Time{
		d(2026, 1, 9), d(2026, 2, 9), d(2026, 3, 10),
		d(2026, 4, 9), d(2026, 5, 11), d(2026, 6, 9),
	}
	got := detectRhythm(dates)
	if got.cadence != CadenceMonthly {
		t.Fatalf("cadence = %v, want monthly", got.cadence)
	}
	if got.anchorDay != 9 {
		t.Errorf("anchorDay = %d, want 9", got.anchorDay)
	}
	if got.postsBy != 11 {
		t.Errorf("postsBy = %d, want 11", got.postsBy)
	}
}
