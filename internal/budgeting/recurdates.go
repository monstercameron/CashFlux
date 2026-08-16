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
//
// The order the tests are applied in matters, and the first draft got it wrong:
// it asked "has this passed?" before ever looking at the period, which reads
// correctly only while you are viewing the live one. Reviewing a CLOSED July in
// August, a bill due Aug 3 was reported "Was due Aug 3" — in danger tone, on a
// period it has nothing to do with. Planning ahead into October, a bill due
// Sep 15 was reported "after this period" when September precedes October. The
// period is asked FIRST now, and a date that falls short of the window has its
// own state instead of being folded into the one that means the opposite.
type RecurDateState string

const (
	// RecurOverdue: inside the period being viewed, the date has passed, and the
	// charge has not been rescheduled. The only state that asks for anything.
	RecurOverdue RecurDateState = "overdue"
	// RecurDueInPeriod: inside the period being viewed and still ahead — money
	// this period's budgets have yet to absorb.
	RecurDueInPeriod RecurDateState = "due-in-period"
	// RecurBeforePeriod: the schedule's next date falls BEFORE the window being
	// viewed. Looking at a future period, the charge lands before you get there;
	// looking at a closed one, it has already come and gone.
	RecurBeforePeriod RecurDateState = "before-period"
	// RecurAfterPeriod: the schedule's next date falls after the window being
	// viewed. Real, and not this period's business.
	RecurAfterPeriod RecurDateState = "after-period"
	// RecurUnscheduled: no next date stored, so nothing can be claimed about it.
	RecurUnscheduled RecurDateState = "unscheduled"
)

// ClassifyRecurDate places a recurring's next-due date relative to the viewed
// period's half-open range first, and only then relative to now.
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
	if periodStart.IsZero() || periodEnd.IsZero() {
		// No window to place it in — the only true statement left is whether the
		// date has passed.
		if next.Before(now) {
			return RecurOverdue
		}
		return RecurDueInPeriod
	}
	switch {
	case next.Before(periodStart):
		return RecurBeforePeriod
	case !next.Before(periodEnd):
		return RecurAfterPeriod
	}
	// Inside the window: only here does "has it passed?" mean anything.
	if next.Before(now) {
		return RecurOverdue
	}
	return RecurDueInPeriod
}
