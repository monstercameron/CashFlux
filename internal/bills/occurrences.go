// SPDX-License-Identifier: MIT

package bills

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// OccurrencesWithin returns every bill occurrence due in [now, until] — unlike
// Upcoming/UpcomingAll, which yield one NEXT occurrence per bill, this projects
// each bill's repeat schedule through the whole window. Callers that plan or
// render across months (the smart pay schedule, the bills calendar) need next
// month's occurrence of a monthly bill to exist as its own item: paying a bill
// ahead means paying NEXT month's occurrence on one of THIS month's paydays,
// which is impossible if the occurrence isn't in the data. Liability statements
// repeat monthly on their due-day (month-end clamped); recurring outflows step
// by their cadence. Occurrence identity is (account, date, name), so two
// occurrences of one bill are distinct items. Iteration is bounded so a
// degenerate imported schedule cannot loop forever.
//
// The result is ALWAYS deduped (DedupeObligations): a liability's own statement
// bill and the monthly recurring flow the household created to pay it are one
// real payment, and every surface that projects a window — the bills calendar,
// the pay-ahead planner, the payday preflight — was listing both. That inflated
// "Total due soon" to $8,814.00, the upcoming counts, and the calendar badges
// (V-sweep C340). Deduping here rather than at the call sites is the point:
// three of the four callers had simply forgotten to, and a correctness rule a
// caller can forget is not a rule.
func OccurrencesWithin(accounts []domain.Account, recurring []domain.Recurring, now, until time.Time) []Bill {
	return DedupeObligations(occurrencesRaw(accounts, recurring, now, until), recurring)
}

// occurrencesRaw projects the schedules without collapsing the dual bill
// identity. Only OccurrencesWithin calls it — the raw list is an intermediate,
// never an answer.
func occurrencesRaw(accounts []domain.Account, recurring []domain.Recurring, now, until time.Time) []Bill {
	end := dateOnly(until)
	var out []Bill
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		if a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		for due, i := NextDue(a.DueDayOfMonth, now), 0; !due.After(end) && i < 60; i++ {
			out = append(out, Bill{
				AccountID: a.ID,
				Name:      a.Name,
				Amount:    a.MinPayment.Abs(),
				DueDate:   due,
				DaysUntil: daysBetween(now, due),
			})
			due = NextDue(a.DueDayOfMonth, due.AddDate(0, 0, 1))
		}
	}
	for _, r := range recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		due, ok := nextRecurringDue(r, now)
		if !ok {
			continue
		}
		for i := 0; !due.After(end) && i < 120; i++ {
			out = append(out, Bill{
				AccountID: "recurring:" + r.ID,
				Name:      r.Label,
				Amount:    r.Amount.Abs(),
				DueDate:   due,
				DaysUntil: daysBetween(now, due),
				Autopay:   r.Autopay,
			})
			next := dateOnly(r.Cadence.Next(due))
			if !next.After(due) {
				break
			}
			due = next
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return billLess(out[i], out[j])
	})
	return out
}
