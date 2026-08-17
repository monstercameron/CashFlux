// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestResolveWindowPresets(t *testing.T) {
	now := time.Date(2026, time.August, 17, 14, 30, 0, 0, time.UTC)
	cases := []struct {
		preset                   string
		start, end, pStart, pEnd time.Time
		label                    string
	}{
		{"this_month", day(2026, 8, 1), day(2026, 9, 1), day(2026, 7, 1), day(2026, 8, 1), "August 2026"},
		{"last_month", day(2026, 7, 1), day(2026, 8, 1), day(2026, 6, 1), day(2026, 7, 1), "July 2026"},
		{"this_quarter", day(2026, 7, 1), day(2026, 10, 1), day(2026, 4, 1), day(2026, 7, 1), "Q3 2026"},
		{"last_quarter", day(2026, 4, 1), day(2026, 7, 1), day(2026, 1, 1), day(2026, 4, 1), "Q2 2026"},
		{"this_year", day(2026, 1, 1), day(2027, 1, 1), day(2025, 1, 1), day(2026, 1, 1), "2026"},
		{"last_year", day(2025, 1, 1), day(2026, 1, 1), day(2024, 1, 1), day(2025, 1, 1), "2025"},
		{"year_to_date", day(2026, 1, 1), day(2026, 8, 18), day(2025, 1, 1), day(2026, 1, 1), "2026 so far"},
		{"last_12_months", day(2025, 9, 1), day(2026, 9, 1), day(2024, 9, 1), day(2025, 9, 1), "Sep 2025 – Aug 2026"},
	}
	for _, tc := range cases {
		w := ResolveWindow(tc.preset, "", "", now)
		if !w.Start.Equal(tc.start) || !w.End.Equal(tc.end) {
			t.Errorf("%s: window = %s..%s, want %s..%s", tc.preset, w.Start, w.End, tc.start, tc.end)
		}
		if !w.PrevStart.Equal(tc.pStart) || !w.PrevEnd.Equal(tc.pEnd) {
			t.Errorf("%s: previous = %s..%s, want %s..%s", tc.preset, w.PrevStart, w.PrevEnd, tc.pStart, tc.pEnd)
		}
		if w.Label != tc.label {
			t.Errorf("%s: label = %q, want %q", tc.preset, w.Label, tc.label)
		}
	}
}

func TestResolveWindowAllTimeHasNoComparison(t *testing.T) {
	now := time.Date(2026, time.August, 17, 9, 0, 0, 0, time.UTC)
	w := ResolveWindow("all", "", "", now)
	if !w.Start.IsZero() {
		t.Fatalf("all-time start = %s, want zero", w.Start)
	}
	if !w.End.Equal(day(2026, 8, 18)) {
		t.Fatalf("all-time end = %s, want 2026-08-18 (through today)", w.End)
	}
	if w.HasPrevious() {
		t.Fatal("all time should not claim a comparison window")
	}
}

func TestResolveWindowExplicitDatesIncludeTheLastDay(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	w := ResolveWindow("this_month", "2026-03-01", "2026-03-31", now)
	if !w.Start.Equal(day(2026, 3, 1)) {
		t.Fatalf("start = %s", w.Start)
	}
	// The half-open end must be April 1st: an exclusive 03-31 would silently
	// drop every transaction dated on the last day of the range.
	if !w.End.Equal(day(2026, 4, 1)) {
		t.Fatalf("end = %s, want 2026-04-01 (the `to` date is inclusive)", w.End)
	}
	// An explicit range compares against the same LENGTH immediately before it,
	// not against the previous calendar month: March's 31 days back from March
	// 1st lands on January 29th, and that is the honest comparison for a range
	// the user drew by hand.
	if !w.PrevStart.Equal(day(2026, 1, 29)) || !w.PrevEnd.Equal(day(2026, 3, 1)) {
		t.Fatalf("previous = %s..%s, want the same 31-day span immediately before", w.PrevStart, w.PrevEnd)
	}
	if w.Label != "2026-03-01 – 2026-03-31" {
		t.Fatalf("label = %q", w.Label)
	}
}

func TestResolveWindowExplicitDatesBeatThePreset(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	w := ResolveWindow("last_year", "2026-06-01", "", now)
	if !w.Start.Equal(day(2026, 6, 1)) || !w.End.Equal(day(2026, 8, 18)) {
		t.Fatalf("window = %s..%s, want 2026-06-01 through today", w.Start, w.End)
	}
}

func TestResolveWindowOpenStart(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	w := ResolveWindow("", "", "2026-02-14", now)
	if !w.Start.IsZero() || !w.End.Equal(day(2026, 2, 15)) {
		t.Fatalf("window = %s..%s", w.Start, w.End)
	}
	if w.HasPrevious() {
		t.Fatal("an open-start window has nothing to compare against")
	}
	if w.Label != "everything up to 2026-02-14" {
		t.Fatalf("label = %q", w.Label)
	}
}

func TestResolveWindowUnknownPresetFallsBackToThisMonth(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	w := ResolveWindow("since_the_dawn_of_time", "", "", now)
	if !w.Start.Equal(day(2026, 8, 1)) || w.Label != "August 2026" {
		t.Fatalf("window = %s (%q), want the current month", w.Start, w.Label)
	}
}

func TestQuarterHelpers(t *testing.T) {
	for _, tc := range []struct {
		in    time.Time
		start time.Time
		label string
	}{
		{day(2026, 1, 1), day(2026, 1, 1), "Q1 2026"},
		{day(2026, 3, 31), day(2026, 1, 1), "Q1 2026"},
		{day(2026, 4, 1), day(2026, 4, 1), "Q2 2026"},
		{day(2026, 12, 31), day(2026, 10, 1), "Q4 2026"},
	} {
		got := QuarterStart(tc.in)
		if !got.Equal(tc.start) {
			t.Errorf("QuarterStart(%s) = %s, want %s", tc.in, got, tc.start)
		}
		if l := QuarterLabel(got); l != tc.label {
			t.Errorf("QuarterLabel(%s) = %q, want %q", got, l, tc.label)
		}
	}
}

func TestParseMonth(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		in   string
		want time.Time
	}{
		{"2026-03", day(2026, 3, 1)},
		{"March 2026", day(2026, 3, 1)},
		{"Mar 2026", day(2026, 3, 1)},
		{"2026-03-19", day(2026, 3, 1)},
		{"March", day(2026, 3, 1)}, // bare month = this year
		{" 2026-03 ", day(2026, 3, 1)},
	} {
		got, ok := ParseMonth(tc.in, now)
		if !ok || !got.Equal(tc.want) {
			t.Errorf("ParseMonth(%q) = %s, %v; want %s", tc.in, got, ok, tc.want)
		}
	}
	if _, ok := ParseMonth("sometime last spring", now); ok {
		t.Fatal("ParseMonth accepted prose")
	}
}
