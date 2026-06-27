// SPDX-License-Identifier: MIT

package safespend

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
)

// BillsDueBefore sums the bills/recurring items due strictly before the cutoff,
// converting each to base-currency minor units via toBase. Items whose due date
// is on or after cutoff are excluded.
func BillsDueBefore(
	accounts []domain.Account,
	recurring []domain.Recurring,
	now time.Time,
	cutoff time.Time,
	toBase func(amount int64, currency string) int64,
) int64 {
	var total int64
	for _, b := range bills.UpcomingAll(accounts, recurring, now) {
		if !b.DueDate.Before(cutoff) {
			continue
		}
		total += toBase(b.Amount.Amount, b.Amount.Currency)
	}
	return total
}

// GoalContributionsProrated sums the monthly contributions needed for active,
// non-archived goals (those with a target date and remaining balance), converting
// each to base-currency minor units via toBase.
func GoalContributionsProrated(
	gs []domain.Goal,
	now time.Time,
	toBase func(amount int64, currency string) int64,
) int64 {
	var total int64
	for _, g := range gs {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, now)
		if err != nil || !ok {
			continue
		}
		total += toBase(needed.Amount, needed.Currency)
	}
	return total
}

// ToBaseFunc returns a converter function that maps (minorAmount, srcCurrency)
// to base-currency minor units using the supplied Rates. Conversion errors
// silently return 0 so callers that aggregate many items stay robust.
func ToBaseFunc(rates currency.Rates) func(int64, string) int64 {
	return func(amount int64, src string) int64 {
		result, err := currency.ConvertBetween(amount, src, rates.Base, rates)
		if err != nil {
			return 0
		}
		return result
	}
}
