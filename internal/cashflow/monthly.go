// SPDX-License-Identifier: MIT

package cashflow

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// DefaultTrailingMonths is the standard look-back window for the monthly cash
// baselines. Three whole months smooths a single unusual month without lagging a
// real change in income or spending.
const DefaultTrailingMonths = 3

// TrailingMonthly returns the average monthly income and expense magnitude (base-
// currency minor units) over the `months` whole calendar months immediately before
// `now`. It is the single shared definition of "money in / money out per month" so
// every surface — the Smart assistant, the Goals pace verdict, the savings sweep —
// derives the SAME figure instead of each rolling its own and quietly disagreeing.
//
// Transfers are excluded (they move money between the household's own accounts; they
// neither earn nor spend it). Amounts in another currency are converted to base; an
// unconvertible amount is treated as absent (0) rather than failing the whole
// calculation. `months` below 1 is clamped to 1.
func TrailingMonthly(txns []domain.Transaction, rates currency.Rates, base string, now time.Time, months int) (income, expense int64) {
	if months < 1 {
		months = 1
	}
	curStart := dateutil.MonthStart(now)
	var inc, exp int64
	for k := 1; k <= months; k++ {
		start := dateutil.AddMonths(curStart, -k)
		end := dateutil.AddMonths(curStart, -k+1)
		for _, t := range txns {
			if t.IsTransfer() || t.Date.Before(start) || !t.Date.Before(end) {
				continue
			}
			b := toBaseMinor(t.Amount.Amount, t.Amount.Currency, base, rates)
			if t.Amount.IsPositive() {
				inc += b
			} else {
				exp += -b
			}
		}
	}
	return inc / int64(months), exp / int64(months)
}

// TrailingMonthlySurplus is average monthly income minus average monthly expense
// over the trailing window — the free cash available each month before it is
// assigned to goals or savings. It can be negative when spending outran income.
func TrailingMonthlySurplus(txns []domain.Transaction, rates currency.Rates, base string, now time.Time, months int) int64 {
	income, expense := TrailingMonthly(txns, rates, base, now, months)
	return income - expense
}

// toBaseMinor converts a minor amount in `from` currency to base minor units,
// treating an empty/base currency as already-base and an unconvertible amount as 0.
func toBaseMinor(amount int64, from, base string, rates currency.Rates) int64 {
	if from == "" || from == base {
		return amount
	}
	v, err := currency.ConvertBetween(amount, from, base, rates)
	if err != nil {
		return 0
	}
	return v
}
