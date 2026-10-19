// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TrueUp thresholds. A drift below these is normal variation and never flagged, so the
// suggestion is a real, sustained gap between the budget and reality — not this month's noise.
const (
	// trueUpMinRatioBps is how far above the limit the learned spend must run before a
	// true-up is worth suggesting: 110% (a persistent 10%+ overshoot).
	trueUpMinRatioBps = 11000
	// trueUpMinDeltaMinor is the smallest absolute raise worth surfacing ($10) so a tiny
	// budget doesn't nag over a couple of dollars.
	trueUpMinDeltaMinor = 10_00
	// trueUpRoundMinor rounds a suggested new limit to the nearest whole currency unit so the
	// proposed figure is clean ("$480", not "$479.63").
	trueUpRoundMinor = 100
	// seasonalMinMonths is the history (in months) required before a same-month-last-year
	// (seasonal) basis is trusted; below it the detector uses a trailing average.
	seasonalMinMonths = 13
	// trueUpTrailingMonths is the trailing window averaged when there isn't enough history
	// for a seasonal read.
	trueUpTrailingMonths = 6
)

// TrueUp is one suggested budget increase: the learned spend has run persistently above the
// current limit, so raising the limit to SuggestedMinor brings the budget back in line with
// reality. Amounts are minor units in the budget's limit currency.
type TrueUp struct {
	Budget            domain.Budget
	CurrentLimitMinor int64
	// LearnedMinor is the monthly spend the history implies (seasonal or trailing average).
	LearnedMinor int64
	// SuggestedMinor is LearnedMinor rounded to a clean figure — the one-tap new limit.
	SuggestedMinor int64
	// BasisMonths is how many months of spend the learned figure averages over (for copy:
	// "ran $480 for 6 months").
	BasisMonths int
	// Seasonal is true when the learned figure came from same-month-last-year data rather
	// than a trailing average.
	Seasonal bool
}

// SuggestTrueUps re-fits each MONTHLY budget against its own spending history and returns the
// budgets whose real spend has drifted persistently above their limit (BG6). It is the pure
// core behind the seasonal auto-budget true-up: the SMART engine wraps these into opt-in,
// dismissable suggestions.
//
// Basis selection per budget:
//   - When at least seasonalMinMonths of history exist, the learned figure is the average of
//     prior same-calendar-month spends (this July vs past Julys) — so a genuinely seasonal
//     category is trued up to its seasonal norm, not a blended average that is wrong every
//     month.
//   - Otherwise the learned figure is the trailing-average monthly spend, mirroring the
//     auto-budget's own recent-spend method.
//
// Only monthly budgets are considered (the true-up compares a monthly limit to monthly
// spend); non-monthly budgets are skipped rather than guessed at. Results are sorted by the
// size of the suggested raise, largest first.
func SuggestTrueUps(budgets []domain.Budget, txns NettedLedger, cats []domain.Category, now time.Time, rates currency.Rates) ([]TrueUp, error) {
	var out []TrueUp
	for _, b := range budgets {
		if b.Period != domain.PeriodMonthly || b.IsSaving() {
			continue
		}
		covers := categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
		learned, months, seasonal, err := learnedMonthlySpend(b, txns, cats, now, rates, covers)
		if err != nil {
			return nil, err
		}
		if months == 0 || learned <= 0 {
			continue
		}
		currentLimit, err := BudgetBaseAt(b, now)
		if err != nil {
			return nil, fmt.Errorf("true-up budget %q current limit: %w", b.ID, err)
		}
		limit := currentLimit.Amount
		if limit <= 0 {
			continue
		}
		threshold, err := money.MulDiv(limit, trueUpMinRatioBps, 10000, money.RoundTowardZero)
		if err != nil {
			return nil, err
		}
		if learned < threshold {
			continue // within normal variation of the current limit
		}
		suggested, err := roundToChecked(learned, trueUpRoundMinor)
		if err != nil {
			return nil, err
		}
		if suggested-limit < trueUpMinDeltaMinor {
			continue
		}
		out = append(out, TrueUp{
			Budget: b, CurrentLimitMinor: limit, LearnedMinor: learned,
			SuggestedMinor: suggested, BasisMonths: months, Seasonal: seasonal,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := out[i].SuggestedMinor - out[i].CurrentLimitMinor
		dj := out[j].SuggestedMinor - out[j].CurrentLimitMinor
		if di != dj {
			return di > dj
		}
		return out[i].Budget.ID < out[j].Budget.ID
	})
	return out, nil
}

// learnedMonthlySpend returns the monthly spend a budget's history implies, the number of
// months averaged, and whether a seasonal (same-month) basis was used. Spend is summed in
// the budget's limit currency over the covered categories, honouring splits.
func learnedMonthlySpend(b domain.Budget, txns NettedLedger, cats []domain.Category, now time.Time, rates currency.Rates, covers map[string]bool) (learned int64, months int, seasonal bool, err error) {
	tracks := func(id string) bool { return b.TracksCategory(id) || covers[id] }
	curStart := dateutil.MonthStart(now)

	// How many completed months of history exist (from the earliest covered expense).
	earliest := time.Time{}
	for _, t := range txns {
		if t.Date.After(dateutil.WallClockUTC(now)) {
			continue
		}
		_, ok, err := ContributionChecked(b, t, tracks)
		if err != nil {
			return 0, 0, false, err
		}
		if !ok {
			continue
		}
		if earliest.IsZero() || t.Date.Before(earliest) {
			earliest = t.Date
		}
	}
	if earliest.IsZero() {
		return 0, 0, false, nil
	}
	historyMonths := monthsBetween(earliest, curStart)

	if historyMonths >= seasonalMinMonths {
		// Seasonal: average prior same-calendar-month spends (exclude the in-progress month).
		var total, count int64
		for k := 12; k <= historyMonths; k += 12 {
			ms := dateutil.AddMonths(curStart, -k)
			me := dateutil.AddMonths(ms, 1)
			sum, err := sumCoveredInRange(b, txns, ms, me, rates, tracks)
			if err != nil {
				return 0, 0, false, err
			}
			total, err = money.Add(total, sum)
			if err != nil {
				return 0, 0, false, err
			}
			count++
		}
		if count > 0 {
			return total / count, int(count), true, nil
		}
	}

	// Trailing average over the completed months in the window (skip the in-progress month).
	window := trueUpTrailingMonths
	if window > historyMonths {
		window = historyMonths
	}
	if window <= 0 {
		return 0, 0, false, nil
	}
	var total int64
	for k := 1; k <= window; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		sum, err := sumCoveredInRange(b, txns, ms, me, rates, tracks)
		if err != nil {
			return 0, 0, false, err
		}
		total, err = money.Add(total, sum)
		if err != nil {
			return 0, 0, false, err
		}
	}
	return total / int64(window), window, false, nil
}

// sumCoveredInRange sums covered expense for a budget in [start, end), in the limit currency.
func sumCoveredInRange(b domain.Budget, txns NettedLedger, start, end time.Time, rates currency.Rates, tracks func(string) bool) (int64, error) {
	amount, err := spentCovered(b, txns, start, end, rates, tracks)
	return amount.Amount, err
}

// monthsBetween returns the whole-month count from a to b (both reduced to first-of-month).
func monthsBetween(a, b time.Time) int {
	return int(b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// roundToChecked rounds v to the nearest step without an overflowing addition.
func roundToChecked(v, step int64) (int64, error) {
	if step <= 1 {
		return v, nil
	}
	units, err := money.MulDiv(v, 1, step, money.RoundHalfAwayFromZero)
	if err != nil {
		return 0, err
	}
	return money.Mul(units, step)
}
