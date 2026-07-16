// SPDX-License-Identifier: MIT

package calendargrid

import (
	"testing"
	"time"
)

// day is a terse UTC-midnight date constructor for the tables.
func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// flatten returns every cell of the grid in row-major order.
func flatten(weeks [][]Day) []Day {
	var out []Day
	for _, w := range weeks {
		out = append(out, w...)
	}
	return out
}

func TestWeekdayOrder(t *testing.T) {
	tests := []struct {
		name      string
		weekStart time.Weekday
		want      []time.Weekday
	}{
		{"sunday-start", time.Sunday, []time.Weekday{
			time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday,
		}},
		{"monday-start", time.Monday, []time.Weekday{
			time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday,
		}},
		{"saturday-start", time.Saturday, []time.Weekday{
			time.Saturday, time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeekdayOrder(tt.weekStart)
			if len(got) != 7 {
				t.Fatalf("len = %d, want 7", len(got))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestMonthShape checks week counts, the first/last grid cell, and the in-month
// span for months that start on different weekdays and have different lengths.
func TestMonthShape(t *testing.T) {
	tests := []struct {
		name       string
		anchor     time.Time
		weekStart  time.Weekday
		wantWeeks  int
		firstCell  time.Time // top-left grid day
		lastCell   time.Time // bottom-right grid day
		inMonth    int       // count of InMonth days (== days in the month)
		firstInDay time.Time // first InMonth day (== the 1st)
		lastInDay  time.Time // last InMonth day (== last of month)
	}{
		{
			// Feb 2015: Feb 1 is a Sunday, 28 days, Sunday-start -> a clean 4-week block.
			name: "feb-2015-4week-sunday", anchor: day(2015, time.February, 15), weekStart: time.Sunday,
			wantWeeks: 4, firstCell: day(2015, time.February, 1), lastCell: day(2015, time.February, 28),
			inMonth: 28, firstInDay: day(2015, time.February, 1), lastInDay: day(2015, time.February, 28),
		},
		{
			// May 2021: May 1 is Saturday, 31 days, Sunday-start -> spills to 6 weeks.
			name: "may-2021-6week-sunday", anchor: day(2021, time.May, 10), weekStart: time.Sunday,
			wantWeeks: 6, firstCell: day(2021, time.April, 25), lastCell: day(2021, time.June, 5),
			inMonth: 31, firstInDay: day(2021, time.May, 1), lastInDay: day(2021, time.May, 31),
		},
		{
			// Jan 2021: Jan 1 is Friday, 31 days, Sunday-start -> 6 weeks.
			name: "jan-2021-sunday", anchor: day(2021, time.January, 1), weekStart: time.Sunday,
			wantWeeks: 6, firstCell: day(2020, time.December, 27), lastCell: day(2021, time.February, 6),
			inMonth: 31, firstInDay: day(2021, time.January, 1), lastInDay: day(2021, time.January, 31),
		},
		{
			// Feb 2024 (leap): Feb 1 is Thursday, 29 days, Sunday-start -> 5 weeks.
			name: "feb-2024-leap-sunday", anchor: day(2024, time.February, 29), weekStart: time.Sunday,
			wantWeeks: 5, firstCell: day(2024, time.January, 28), lastCell: day(2024, time.March, 2),
			inMonth: 29, firstInDay: day(2024, time.February, 1), lastInDay: day(2024, time.February, 29),
		},
		{
			// Feb 2021 (non-leap): Feb 1 is Monday, 28 days, Sunday-start -> 5 weeks (offset 1).
			name: "feb-2021-nonleap-sunday", anchor: day(2021, time.February, 14), weekStart: time.Sunday,
			wantWeeks: 5, firstCell: day(2021, time.January, 31), lastCell: day(2021, time.March, 6),
			inMonth: 28, firstInDay: day(2021, time.February, 1), lastInDay: day(2021, time.February, 28),
		},
		{
			// Same Feb 2021 but Monday-start: Feb 1 Monday -> offset 0, exactly 4 weeks.
			name: "feb-2021-nonleap-monday-4week", anchor: day(2021, time.February, 14), weekStart: time.Monday,
			wantWeeks: 4, firstCell: day(2021, time.February, 1), lastCell: day(2021, time.February, 28),
			inMonth: 28, firstInDay: day(2021, time.February, 1), lastInDay: day(2021, time.February, 28),
		},
		{
			// Oct 2021: Oct 1 is Friday, 31 days, Monday-start -> offset 4, a clean 5 weeks
			// (Oct 31 is a Sunday, so no trailing pad).
			name: "oct-2021-monday", anchor: day(2021, time.October, 1), weekStart: time.Monday,
			wantWeeks: 5, firstCell: day(2021, time.September, 27), lastCell: day(2021, time.October, 31),
			inMonth: 31, firstInDay: day(2021, time.October, 1), lastInDay: day(2021, time.October, 31),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weeks := Month(tt.anchor, tt.weekStart, time.Time{})
			if len(weeks) != tt.wantWeeks {
				t.Fatalf("weeks = %d, want %d", len(weeks), tt.wantWeeks)
			}
			cells := flatten(weeks)
			if len(cells) != tt.wantWeeks*7 {
				t.Fatalf("cells = %d, want %d", len(cells), tt.wantWeeks*7)
			}
			// Every row is exactly 7 days.
			for i, w := range weeks {
				if len(w) != 7 {
					t.Errorf("week %d has %d days, want 7", i, len(w))
				}
			}
			// First grid cell must sit on the requested week-start.
			if got := cells[0].Date.Weekday(); got != tt.weekStart {
				t.Errorf("first cell weekday = %v, want %v", got, tt.weekStart)
			}
			if !cells[0].Date.Equal(tt.firstCell) {
				t.Errorf("first cell = %v, want %v", cells[0].Date, tt.firstCell)
			}
			if last := cells[len(cells)-1].Date; !last.Equal(tt.lastCell) {
				t.Errorf("last cell = %v, want %v", last, tt.lastCell)
			}

			// Grid days must be strictly consecutive.
			for i := 1; i < len(cells); i++ {
				want := cells[i-1].Date.AddDate(0, 0, 1)
				if !cells[i].Date.Equal(want) {
					t.Errorf("cell %d = %v, want %v (consecutive)", i, cells[i].Date, want)
				}
			}

			// InMonth count + boundaries.
			var inCount int
			var firstIn, lastIn time.Time
			for _, c := range cells {
				if c.InMonth {
					if firstIn.IsZero() {
						firstIn = c.Date
					}
					lastIn = c.Date
					inCount++
				}
			}
			if inCount != tt.inMonth {
				t.Errorf("InMonth count = %d, want %d", inCount, tt.inMonth)
			}
			if !firstIn.Equal(tt.firstInDay) {
				t.Errorf("first InMonth day = %v, want %v", firstIn, tt.firstInDay)
			}
			if !lastIn.Equal(tt.lastInDay) {
				t.Errorf("last InMonth day = %v, want %v", lastIn, tt.lastInDay)
			}

			// The InMonth run must be contiguous (no gaps, no leading/trailing month
			// days flagged in-month).
			seenIn := false
			doneIn := false
			for _, c := range cells {
				switch {
				case c.InMonth && !seenIn:
					seenIn = true
				case !c.InMonth && seenIn && !doneIn:
					doneIn = true
				case c.InMonth && doneIn:
					t.Errorf("InMonth run is not contiguous at %v", c.Date)
				}
			}
		})
	}
}

// TestIsToday verifies today-flagging by calendar date, including that only one
// cell is flagged, that time-of-day is ignored, and that an out-of-range/zero
// today flags nothing.
func TestIsToday(t *testing.T) {
	anchor := day(2021, time.May, 10)

	t.Run("in-month today, time ignored", func(t *testing.T) {
		today := time.Date(2021, time.May, 10, 13, 45, 0, 0, time.UTC)
		weeks := Month(anchor, time.Sunday, today)
		var flagged int
		for _, c := range flatten(weeks) {
			if c.IsToday {
				flagged++
				if c.Date.Day() != 10 || c.Date.Month() != time.May {
					t.Errorf("wrong day flagged: %v", c.Date)
				}
			}
		}
		if flagged != 1 {
			t.Errorf("flagged %d cells, want 1", flagged)
		}
	})

	t.Run("today in padding day of neighbouring month", func(t *testing.T) {
		// Apr 25 2021 is the top-left padding cell of May's Sunday-start grid.
		today := day(2021, time.April, 25)
		weeks := Month(anchor, time.Sunday, today)
		c := weeks[0][0]
		if !c.IsToday || c.InMonth {
			t.Errorf("padding cell %v: IsToday=%v InMonth=%v, want today-but-not-in-month", c.Date, c.IsToday, c.InMonth)
		}
	})

	t.Run("today outside the grid flags nothing", func(t *testing.T) {
		weeks := Month(anchor, time.Sunday, day(2019, time.January, 1))
		for _, c := range flatten(weeks) {
			if c.IsToday {
				t.Errorf("unexpected today flag on %v", c.Date)
			}
		}
	})

	t.Run("zero today flags nothing", func(t *testing.T) {
		weeks := Month(anchor, time.Sunday, time.Time{})
		for _, c := range flatten(weeks) {
			if c.IsToday {
				t.Errorf("unexpected today flag on %v with zero today", c.Date)
			}
		}
	})
}

// TestNormalizedToMidnight checks that cells are emitted at 00:00 regardless of
// the anchor's time-of-day.
func TestNormalizedToMidnight(t *testing.T) {
	anchor := time.Date(2021, time.May, 10, 17, 30, 45, 123, time.UTC)
	weeks := Month(anchor, time.Sunday, time.Time{})
	for _, c := range flatten(weeks) {
		h, m, s := c.Date.Clock()
		if h != 0 || m != 0 || s != 0 || c.Date.Nanosecond() != 0 {
			t.Errorf("cell %v not at midnight", c.Date)
		}
	}
}
