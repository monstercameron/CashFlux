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
func OccurrencesWithin(accounts []domain.Account, recurring []domain.Recurring, now, until time.Time) []Bill {
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
