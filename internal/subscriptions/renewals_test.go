// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"
)

func sub(name string, renewal time.Time) Subscription {
	return Subscription{Name: name, NextRenewal: renewal}
}

func TestUpcomingRenewals(t *testing.T) {
	now := d(2026, time.June, 10)
	subs := []Subscription{
		sub("Later", d(2026, time.June, 25)), // outside 7-day window
		sub("Soon", d(2026, time.June, 12)),  // within
		sub("Today", d(2026, time.June, 10)), // within (today)
		sub("Past", d(2026, time.June, 1)),   // already passed → excluded
		sub("Edge", d(2026, time.June, 17)),  // exactly now+7 → within
	}
	got := UpcomingRenewals(subs, 7, now)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3 (Today, Soon, Edge): %+v", len(got), got)
	}
	// Soonest first: Today (10), Soon (12), Edge (17).
	if got[0].Name != "Today" || got[1].Name != "Soon" || got[2].Name != "Edge" {
		t.Errorf("order = %s,%s,%s; want Today,Soon,Edge", got[0].Name, got[1].Name, got[2].Name)
	}
}

func TestUpcomingRenewalsDefaultWindow(t *testing.T) {
	now := d(2026, time.June, 10)
	subs := []Subscription{sub("A", d(2026, time.June, 16))} // within default 7
	if got := UpcomingRenewals(subs, 0, now); len(got) != 1 {
		t.Errorf("non-positive window should default to 7 days, got %d", len(got))
	}
}

func TestUpcomingRenewalsNone(t *testing.T) {
	now := d(2026, time.June, 10)
	subs := []Subscription{sub("A", d(2026, time.August, 1))}
	if got := UpcomingRenewals(subs, 7, now); len(got) != 0 {
		t.Errorf("far renewal should be excluded, got %+v", got)
	}
}
