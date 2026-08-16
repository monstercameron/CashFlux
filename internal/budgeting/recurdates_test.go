// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Viewing the CURRENT period is the easy case: inside the window, "has it
// passed?" is the whole question.
func TestClassifyRecurDateInsideTheCurrentPeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	augStart, augEnd := day(2026, time.August, 1), day(2026, time.September, 1)

	cases := []struct {
		name string
		next time.Time
		want RecurDateState
	}{
		{"later this month", day(2026, time.August, 20), RecurDueInPeriod},
		{"today", now, RecurDueInPeriod},
		{"earlier this month, unpaid", day(2026, time.August, 3), RecurOverdue},
		{"next month", day(2026, time.September, 2), RecurAfterPeriod},
		{"last month", day(2026, time.July, 20), RecurBeforePeriod},
		{"no date stored", time.Time{}, RecurUnscheduled},
	}
	for _, c := range cases {
		if got := ClassifyRecurDate(c.next, now, augStart, augEnd); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Reviewing a CLOSED period. The first draft asked "has it passed?" before
// looking at the window, so a bill due Aug 3 was reported "Was due Aug 3" — in
// danger tone — on a July page it has nothing to do with.
func TestClassifyRecurDateViewingAClosedPeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	julStart, julEnd := day(2026, time.July, 1), day(2026, time.August, 1)

	cases := []struct {
		name string
		next time.Time
		want RecurDateState
	}{
		{"due after July, already passed today", day(2026, time.August, 3), RecurAfterPeriod},
		{"due after July, still ahead", day(2026, time.September, 1), RecurAfterPeriod},
		{"inside July and long gone", day(2026, time.July, 3), RecurOverdue},
		{"before July entirely", day(2026, time.June, 3), RecurBeforePeriod},
	}
	for _, c := range cases {
		if got := ClassifyRecurDate(c.next, now, julStart, julEnd); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Planning a FUTURE period. The first draft reported a September date as "after
// this period" while viewing October — chronologically backwards.
func TestClassifyRecurDateViewingAFuturePeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	octStart, octEnd := day(2026, time.October, 1), day(2026, time.November, 1)

	if got := ClassifyRecurDate(day(2026, time.September, 15), now, octStart, octEnd); got != RecurBeforePeriod {
		t.Errorf("a September date while viewing October = %q, want before-period", got)
	}
	if got := ClassifyRecurDate(day(2026, time.October, 15), now, octStart, octEnd); got != RecurDueInPeriod {
		t.Errorf("an October date while viewing October = %q, want due-in-period", got)
	}
	if got := ClassifyRecurDate(day(2026, time.November, 2), now, octStart, octEnd); got != RecurAfterPeriod {
		t.Errorf("a November date while viewing October = %q, want after-period", got)
	}
	// Nothing inside a future period can be overdue — the period has not happened.
	for _, d := range []time.Time{octStart, day(2026, time.October, 31)} {
		if got := ClassifyRecurDate(d, now, octStart, octEnd); got == RecurOverdue {
			t.Errorf("%v inside a future period was called overdue", d)
		}
	}
}

// The end of the range is exclusive: the first instant of the next period is not
// in this one. Getting this wrong puts next month's rent in this month's list.
func TestClassifyRecurDateRangeIsHalfOpen(t *testing.T) {
	now := day(2026, time.August, 2)
	start, end := day(2026, time.August, 1), day(2026, time.September, 1)

	if got := ClassifyRecurDate(end, now, start, end); got != RecurAfterPeriod {
		t.Errorf("the range's end instant = %q, want after-period", got)
	}
	if got := ClassifyRecurDate(end.AddDate(0, 0, -1), now, start, end); got != RecurDueInPeriod {
		t.Errorf("the last day of the period = %q, want due-in-period", got)
	}
	if got := ClassifyRecurDate(start, now, start, end); got != RecurOverdue {
		t.Errorf("the range's first instant, already past = %q, want overdue", got)
	}
}

// Without a period, the honest answer is the today-only one.
func TestClassifyRecurDateWithoutAPeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	if got := ClassifyRecurDate(day(2026, time.December, 1), now, time.Time{}, time.Time{}); got != RecurDueInPeriod {
		t.Errorf("no period, future date = %q, want due-in-period (simply ahead)", got)
	}
	if got := ClassifyRecurDate(day(2026, time.January, 1), now, time.Time{}, time.Time{}); got != RecurOverdue {
		t.Errorf("no period, past date = %q, want overdue", got)
	}
}
