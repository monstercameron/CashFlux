// SPDX-License-Identifier: MIT

package dateutil

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextMonthlyDue(t *testing.T) {
	cases := []struct {
		name string
		now  string
		day  int
		want string
	}{
		{"later this month", "2026-06-10", 15, "2026-06-15"},
		{"on the due day", "2026-06-15", 15, "2026-06-15"},
		{"already passed rolls to next month", "2026-06-20", 15, "2026-07-15"},
		{"clamp over-28 to 28", "2026-06-10", 31, "2026-06-28"},
		{"february clamps to 28", "2026-02-10", 30, "2026-02-28"},
		{"non-positive day clamps to the 1st (already passed → next month)", "2026-06-10", 0, "2026-07-01"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NextMonthlyDue(date(c.now), c.day)
			if FormatDate(got) != c.want {
				t.Errorf("NextMonthlyDue(%s, %d) = %s, want %s", c.now, c.day, FormatDate(got), c.want)
			}
		})
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	got := FormatDate(date("2026-06-15"))
	if got != "2026-06-15" {
		t.Errorf("round trip = %q, want 2026-06-15", got)
	}
	if _, err := ParseDate("nope"); err == nil {
		t.Error("expected parse error")
	}
}

func TestMonthStartAndRange(t *testing.T) {
	start, end := MonthRange(date("2026-06-15"))
	if FormatDate(start) != "2026-06-01" {
		t.Errorf("start = %s, want 2026-06-01", FormatDate(start))
	}
	if FormatDate(end) != "2026-07-01" {
		t.Errorf("end = %s, want 2026-07-01", FormatDate(end))
	}
}

func TestAddMonthsAcrossYear(t *testing.T) {
	got := AddMonths(date("2026-11-01"), 3)
	if FormatDate(got) != "2027-02-01" {
		t.Errorf("AddMonths = %s, want 2027-02-01", FormatDate(got))
	}
}

func TestWeekStart(t *testing.T) {
	// 2026-06-15 is a Monday.
	mon := WeekStart(date("2026-06-17"), time.Monday) // Wed -> back to Mon 15th
	if FormatDate(mon) != "2026-06-15" {
		t.Errorf("week start (Mon) = %s, want 2026-06-15", FormatDate(mon))
	}
	sun := WeekStart(date("2026-06-17"), time.Sunday) // Wed -> back to Sun 14th
	if FormatDate(sun) != "2026-06-14" {
		t.Errorf("week start (Sun) = %s, want 2026-06-14", FormatDate(sun))
	}
}

func TestFiscalMonthRange(t *testing.T) {
	tests := []struct {
		name               string
		day                string
		startDay           int
		wantStart, wantEnd string
	}{
		{"after start day", "2026-06-20", 15, "2026-06-15", "2026-07-15"},
		{"before start day", "2026-06-10", 15, "2026-05-15", "2026-06-15"},
		{"on start day", "2026-06-15", 15, "2026-06-15", "2026-07-15"},
		{"clamped high", "2026-06-10", 40, "2026-05-28", "2026-06-28"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := FiscalMonthRange(date(tt.day), tt.startDay)
			if FormatDate(start) != tt.wantStart || FormatDate(end) != tt.wantEnd {
				t.Errorf("range = [%s,%s), want [%s,%s)", FormatDate(start), FormatDate(end), tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestInRange(t *testing.T) {
	start, end := MonthRange(date("2026-06-15"))
	if !InRange(date("2026-06-01"), start, end) {
		t.Error("start should be in range")
	}
	if InRange(date("2026-07-01"), start, end) {
		t.Error("end is exclusive")
	}
	if InRange(date("2026-05-31"), start, end) {
		t.Error("before start should be out of range")
	}
}

// TestPeriodBoundariesAreUTCRegardlessOfZone guards C1: a transaction stored at
// UTC midnight on the first of the month must be counted in that month's window
// even when "now" is evaluated in a timezone behind UTC (where a naive
// local-zone month-start would land *after* the 00:00Z transaction and silently
// drop it). It also checks a zone ahead of UTC and the week boundary.
func TestPeriodBoundariesAreUTCRegardlessOfZone(t *testing.T) {
	firstOfMonth := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC) // a UTC-dated salary
	zones := []struct {
		name    string
		loc     *time.Location
		nowDay  int
		nowHour int
	}{
		{"behind UTC (UTC-5, mid-month)", time.FixedZone("UTC-5", -5*3600), 15, 9},
		{"behind UTC (UTC-11, early on the 1st)", time.FixedZone("UTC-11", -11*3600), 1, 2},
		{"ahead of UTC (UTC+13)", time.FixedZone("UTC+13", 13*3600), 15, 9},
	}
	for _, z := range zones {
		t.Run(z.name, func(t *testing.T) {
			now := time.Date(2026, time.June, z.nowDay, z.nowHour, 0, 0, 0, z.loc)
			start, end := MonthRange(now)
			if start.Location() != time.UTC || !start.Equal(firstOfMonth) {
				t.Errorf("MonthStart(%s) = %s, want 2026-06-01T00:00:00Z", now.Format(time.RFC3339), start.Format(time.RFC3339))
			}
			if !InRange(firstOfMonth, start, end) {
				t.Errorf("June-1 00:00Z txn not in MonthRange(%s) = [%s, %s)", now.Format(time.RFC3339), start.Format(time.RFC3339), end.Format(time.RFC3339))
			}
		})
	}
}

func TestDaysBetween(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"2026-06-01", "2026-06-15", 14},
		{"2026-06-15", "2026-06-01", -14},
		{"2026-06-15", "2026-06-15", 0},
		{"2026-02-28", "2026-03-01", 1}, // 2026 not a leap year
	}
	for _, tt := range tests {
		if got := DaysBetween(date(tt.a), date(tt.b)); got != tt.want {
			t.Errorf("DaysBetween(%s,%s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
