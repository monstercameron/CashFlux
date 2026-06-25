// SPDX-License-Identifier: MIT

package cashflow

import (
	"testing"
	"time"
)

func TestDipDate(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		p         Projection
		wantTime  time.Time
		wantFound bool
	}{
		{
			name:      "breach=-1 (no breach) — returns false",
			p:         Projection{BreachDay: -1},
			wantTime:  time.Time{},
			wantFound: false,
		},
		{
			name:      "breach=0 (today) — returns from itself",
			p:         Projection{BreachDay: 0},
			wantTime:  from,
			wantFound: true,
		},
		{
			name:      "breach=5 — returns from+5 days",
			p:         Projection{BreachDay: 5},
			wantTime:  time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
			wantFound: true,
		},
		{
			name:      "breach at month boundary",
			p:         Projection{BreachDay: 30},
			wantTime:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := DipDate(tc.p, from)
			if found != tc.wantFound {
				t.Errorf("found = %v, want %v", found, tc.wantFound)
			}
			if found && !got.Equal(tc.wantTime) {
				t.Errorf("date = %v, want %v", got, tc.wantTime)
			}
			if !found && !got.IsZero() {
				t.Errorf("expected zero time when not found, got %v", got)
			}
		})
	}
}

func TestPaydayBalance(t *testing.T) {
	// Build a small projection: 5 days with increasing balances.
	p := Projection{
		Daily: []DailyBalance{
			{Day: 0, Balance: 1000},
			{Day: 1, Balance: 900},
			{Day: 2, Balance: 800},
			{Day: 3, Balance: 1500},
			{Day: 4, Balance: 1400},
		},
		BreachDay: -1,
	}

	tests := []struct {
		name    string
		p       Projection
		horizon int
		want    int64
	}{
		{
			name:    "horizon within bounds — exact index",
			p:       p,
			horizon: 2,
			want:    800,
		},
		{
			name:    "horizon == last index",
			p:       p,
			horizon: 4,
			want:    1400,
		},
		{
			name:    "horizon > len — clamps to last",
			p:       p,
			horizon: 100,
			want:    1400,
		},
		{
			name:    "horizon == len — clamps to last",
			p:       p,
			horizon: 5,
			want:    1400,
		},
		{
			name:    "horizon == 0 — first day",
			p:       p,
			horizon: 0,
			want:    1000,
		},
		{
			name:    "empty Daily — returns 0",
			p:       Projection{BreachDay: -1},
			horizon: 10,
			want:    0,
		},
		{
			name:    "negative horizon — returns 0",
			p:       p,
			horizon: -1,
			want:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PaydayBalance(tc.p, tc.horizon)
			if got != tc.want {
				t.Errorf("PaydayBalance(horizon=%d) = %d, want %d", tc.horizon, got, tc.want)
			}
		})
	}
}
