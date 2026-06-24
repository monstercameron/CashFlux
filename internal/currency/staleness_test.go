// SPDX-License-Identifier: MIT

package currency

import (
	"testing"
	"time"
)

func TestRateStale(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	max := DefaultRateMaxAge
	cases := []struct {
		name      string
		updatedAt time.Time
		want      bool
	}{
		{"zero unknown is not stale", time.Time{}, false},
		{"just set is fresh", now.Add(-1 * time.Hour), false},
		{"29 days is fresh", now.Add(-29 * 24 * time.Hour), false},
		{"31 days is stale", now.Add(-31 * 24 * time.Hour), true},
		{"future is not stale", now.Add(24 * time.Hour), false},
	}
	for _, c := range cases {
		if got := RateStale(c.updatedAt, now, max); got != c.want {
			t.Errorf("%s: RateStale = %v, want %v", c.name, got, c.want)
		}
	}
}
