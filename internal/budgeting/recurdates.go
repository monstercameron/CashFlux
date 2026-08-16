// SPDX-License-Identifier: MIT

package budgeting

import "time"

// RecurDateState classifies a recurring charge's stored next-due date against
// two reference points a budget page has at once: TODAY, and the period the user
// is currently looking at (C609).
//
// The budgets page listed every recurring as "Next <date>" with no reference
// point, so a July view could show "Next Jul 3, 2026" beside "Next Sep 1, 2026"
// and nothing said whether either had happened, was about to, or belonged to the
// period on screen at all. Those are three different facts and only one of them
// is "next".
type RecurDateState string

const (
	// RecurOverdue: the date has passed and the charge has not been rescheduled.
	// The only state that asks the user for anything.
	RecurOverdue RecurDateState = "overdue"
	// RecurDueInPeriod: still ahead, and lands inside the period being viewed —
	// money this period's budgets still have to absorb.
	RecurDueInPeriod RecurDateState = "due-in-period"
	// RecurAfterPeriod: still ahead, but lands after the period being viewed. It
	// is real, and it is not this period's problem.
	RecurAfterPeriod RecurDateState = "after-period"
	// RecurUnscheduled: no next date stored, so nothing can be claimed about it.
	RecurUnscheduled RecurDateState = "unscheduled"
)

// ClassifyRecurDate places a recurring's next-due date relative to now and to
// the viewed period's half-open range.
//
// A zero date is Unscheduled rather than "overdue since year zero" — the
// distinction matters, because one is a question for the user and the other is
// missing data.
//
// The period bounds are optional: with a zero range the answer is the honest
// today-only one (overdue or ahead), which is what a surface with no period
// context should say.
func ClassifyRecurDate(next, now, periodStart, periodEnd time.Time) RecurDateState {
	if next.IsZero() {
		return RecurUnscheduled
	}
	if next.Before(now) {
		return RecurOverdue
	}
	if periodStart.IsZero() || periodEnd.IsZero() {
		return RecurDueInPeriod // no period to compare against; it is simply ahead
	}
	if next.Before(periodStart) || !next.Before(periodEnd) {
		return RecurAfterPeriod
	}
	return RecurDueInPeriod
}
