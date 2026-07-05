// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"
)

// TestLapsed pins the active/lapsed boundary: a pattern whose next renewal is
// slightly past stays active (billing wobble), one more than a full cadence
// interval + grace past is lapsed — the rule that keeps a layoff-era COBRA
// premium out of the live subscriptions list years later.
func TestLapsed(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		cadence Cadence
		next    time.Time
		want    bool
	}{
		{"future renewal", CadenceMonthly, now.AddDate(0, 0, 10), false},
		{"a week overdue", CadenceMonthly, now.AddDate(0, 0, -7), false},
		{"just inside grace", CadenceMonthly, now.AddDate(0, 0, -44), false},
		{"well past monthly", CadenceMonthly, now.AddDate(0, 0, -46), true},
		{"COBRA from 2023", CadenceMonthly, time.Date(2023, 6, 4, 0, 0, 0, 0, time.UTC), true},
		{"yearly, 6 months over", CadenceYearly, now.AddDate(0, -6, 0), false},
		{"yearly, 14 months over", CadenceYearly, now.AddDate(0, -14, 0), true},
		{"weekly, a month over", CadenceWeekly, now.AddDate(0, -1, 0), true},
	}
	for _, tc := range cases {
		s := Subscription{Cadence: tc.cadence, NextRenewal: tc.next}
		if got := s.Lapsed(now); got != tc.want {
			t.Errorf("%s: Lapsed = %v, want %v", tc.name, got, tc.want)
		}
	}
}
