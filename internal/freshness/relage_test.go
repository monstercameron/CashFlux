// SPDX-License-Identifier: MIT

package freshness

import (
	"testing"
	"time"
)

func TestRelAge(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero time", time.Time{}, ""},
		{"future (clock skew)", now.Add(2 * time.Minute), ""},
		{"seconds ago", now.Add(-30 * time.Second), "now"},
		{"minutes ago", now.Add(-4 * time.Minute), "4m"},
		{"just under an hour", now.Add(-59 * time.Minute), "59m"},
		{"hours ago", now.Add(-3 * time.Hour), "3h"},
		{"days ago", now.Add(-12 * 24 * time.Hour), "12d"},
		{"months ago", now.Add(-75 * 24 * time.Hour), "2mo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelAge(tc.t, now); got != tc.want {
				t.Fatalf("RelAge = %q, want %q", got, tc.want)
			}
		})
	}
}
