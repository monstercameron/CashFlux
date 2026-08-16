// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestClassifyRecurDate(t *testing.T) {
	now := day(2026, time.August, 16)
	// The user is looking at the CLOSED July period — the case the ticket reports.
	julStart, julEnd := day(2026, time.July, 1), day(2026, time.August, 1)

	cases := []struct {
		name string
		next time.Time
		want RecurDateState
	}{
		// "Next Jul 3, 2026" on a July view in August: that date is gone.
		{"passed while viewing a closed period", day(2026, time.July, 3), RecurOverdue},
		// "Next Sep 1, 2026" beside it: real, ahead, and nothing to do with July.
		{"ahead but after the viewed period", day(2026, time.September, 1), RecurAfterPeriod},
		// Today itself, while viewing a period that has already closed: not
		// overdue (it has not passed), and not this period's business either.
		{"today, viewing a closed period", now, RecurAfterPeriod},
		{"no date stored", time.Time{}, RecurUnscheduled},
	}
	for _, c := range cases {
		if got := ClassifyRecurDate(c.next, now, julStart, julEnd); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Viewing the CURRENT period, a date inside it is this period's business.
func TestClassifyRecurDateInsideTheCurrentPeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	augStart, augEnd := day(2026, time.August, 1), day(2026, time.September, 1)

	if got := ClassifyRecurDate(day(2026, time.August, 20), now, augStart, augEnd); got != RecurDueInPeriod {
		t.Errorf("a date later this month = %q, want due-in-period", got)
	}
	if got := ClassifyRecurDate(day(2026, time.September, 2), now, augStart, augEnd); got != RecurAfterPeriod {
		t.Errorf("a date next month = %q, want after-period", got)
	}
	if got := ClassifyRecurDate(day(2026, time.August, 3), now, augStart, augEnd); got != RecurOverdue {
		t.Errorf("a date earlier this month = %q, want overdue", got)
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

// Today, while viewing the CURRENT period, is still due — the counterpart of the
// closed-period case above, and the reason the boundary is "before now" rather
// than "not after now".
func TestClassifyRecurDateTodayInTheCurrentPeriod(t *testing.T) {
	now := day(2026, time.August, 16)
	start, end := day(2026, time.August, 1), day(2026, time.September, 1)
	if got := ClassifyRecurDate(now, now, start, end); got != RecurDueInPeriod {
		t.Errorf("a charge due today = %q, want due-in-period", got)
	}
}
