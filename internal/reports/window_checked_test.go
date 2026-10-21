// SPDX-License-Identifier: MIT

package reports

import (
	"strings"
	"testing"
	"time"
)

func TestMonthlyBoundsCheckedRetainsExactPartialAndCalendarEndpoints(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	w, err := ResolveWindowChecked("", "2026-08-15", "2026-09-20", now)
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := MonthlyBoundsChecked(w, time.Time{}, 36)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-08-15", "2026-09-01", "2026-09-21"}
	if len(bounds) != len(want) {
		t.Fatalf("bounds=%v", bounds)
	}
	for i, day := range want {
		if bounds[i].Format("2006-01-02") != day {
			t.Fatalf("bound %d=%v want %s", i, bounds[i], day)
		}
	}
	if !HasPartialMonths(bounds) {
		t.Fatal("partial endpoint hidden")
	}
	full, _ := ResolveWindowChecked("this_month", "", "", now)
	bounds, err = MonthlyBoundsChecked(full, time.Time{}, 36)
	if err != nil || len(bounds) != 2 || HasPartialMonths(bounds) {
		t.Fatalf("full month=%v %v", bounds, err)
	}
	all := AskedWindow{End: now}
	earliest := now.AddDate(0, -1, -3)
	bounds, err = MonthlyBoundsChecked(all, earliest, 36)
	if err != nil || !bounds[0].Equal(earliest) || !bounds[len(bounds)-1].Equal(now) {
		t.Fatalf("all-time endpoints=%v %v", bounds, err)
	}
}

func TestMonthlyBoundsCheckedCapAndInvalidWindowAreExplicit(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	bounds, err := MonthlyBoundsChecked(AskedWindow{Start: start, End: start.AddDate(0, 36, 0)}, time.Time{}, 36)
	if err != nil || len(bounds) != 37 {
		t.Fatalf("36 buckets rejected=%d %v", len(bounds), err)
	}
	bounds, err = MonthlyBoundsChecked(AskedWindow{Start: start, End: start.AddDate(0, 37, 0)}, time.Time{}, 36)
	if bounds != nil || err == nil || !strings.Contains(err.Error(), "narrower") {
		t.Fatalf("37 buckets silently truncated=%v %v", bounds, err)
	}
	for _, tc := range []struct{ period, from, to string }{{"bad", "", ""}, {"", "bad-date", "2026-08-31"}, {"", "2026-09-02", "2026-08-31"}} {
		if _, err := ResolveWindowChecked(tc.period, tc.from, tc.to, start); err == nil {
			t.Fatalf("invalid window accepted: %+v", tc)
		}
	}
}
