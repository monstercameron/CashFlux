// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SuggestionDirection describes how a recommended limit compares with the
// current one.
type SuggestionDirection string

const (
	SuggestionRaise SuggestionDirection = "raise"
	SuggestionLower SuggestionDirection = "lower"
	SuggestionKeep  SuggestionDirection = "keep"
)

// SuggestionMonth is one historical month considered by a recommendation.
type SuggestionMonth struct {
	Month  string
	Amount int64
}

// LimitRecommendation is a history-based limit suggestion with its evidence.
type LimitRecommendation struct {
	CurrentLimit int64
	Suggested    int64
	Direction    SuggestionDirection
	HasEvidence  bool
	Projection   int64
	Median       int64
	MonthsUsed   []SuggestionMonth
	Outliers     []SuggestionMonth
}

// RecommendLimit estimates a budget limit from trailing full months, excludes
// values farther than three median absolute deviations from the median, and
// reports its direction and supporting months. A projected current-period
// amount above the current limit prevents a lower recommendation: the suggestion
// is raised to at least that projection. currentLimit and currentProjection are
// base-currency minor units on the same cadence as the monthly history.
func RecommendLimit(covers map[string]bool, txns []domain.Transaction, now time.Time, months int, currentLimit, currentProjection int64, rates currency.Rates) (LimitRecommendation, error) {
	values, err := monthlyCoveredSpend(covers, txns, now, months, rates)
	if err != nil {
		return LimitRecommendation{}, err
	}
	if len(values) == 0 {
		return LimitRecommendation{CurrentLimit: currentLimit, Suggested: currentLimit, Direction: SuggestionKeep, Projection: currentProjection}, nil
	}
	used, outliers, median, err := excludeSpendOutliers(values)
	if err != nil {
		return LimitRecommendation{}, err
	}
	recommended := median
	if currentProjection > currentLimit && recommended < currentProjection {
		recommended = currentProjection
	}
	direction := SuggestionKeep
	switch {
	case recommended > currentLimit:
		direction = SuggestionRaise
	case recommended < currentLimit:
		direction = SuggestionLower
	}
	if currentProjection > currentLimit && direction == SuggestionLower {
		return LimitRecommendation{}, fmt.Errorf("projected spend exceeds current limit but recommendation would lower it")
	}
	return LimitRecommendation{CurrentLimit: currentLimit, Suggested: recommended, Direction: direction, HasEvidence: true, Projection: currentProjection, Median: median, MonthsUsed: used, Outliers: outliers}, nil
}

// RecommendBudgetLimit normalizes the budget's current limit to monthly
// base-currency minor units before deriving a recommendation. projectedMonthly
// must also be in rates.Base and stated as a monthly amount.
func RecommendBudgetLimit(budget domain.Budget, covers map[string]bool, txns []domain.Transaction, now time.Time, months int, projectedMonthly money.Money, rates currency.Rates) (LimitRecommendation, error) {
	limit := budget.Limit
	if limit.Currency == "" {
		limit = money.New(limit.Amount, rates.Base)
	}
	monthly, err := PeriodEquivalent(limit, budget.Period, domain.PeriodMonthly)
	if err != nil {
		return LimitRecommendation{}, fmt.Errorf("budget %q limit: %w", budget.ID, err)
	}
	monthly, err = rates.Convert(monthly, rates.Base)
	if err != nil {
		return LimitRecommendation{}, fmt.Errorf("budget %q limit currency conversion: %w", budget.ID, err)
	}
	projection, err := rates.Convert(projectedMonthly, rates.Base)
	if err != nil {
		return LimitRecommendation{}, fmt.Errorf("budget %q projected spend currency conversion: %w", budget.ID, err)
	}
	return RecommendLimit(covers, txns, now, months, monthly.Amount, projection.Amount, rates)
}

func monthlyCoveredSpend(covers map[string]bool, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) ([]SuggestionMonth, error) {
	if len(covers) == 0 || months <= 0 {
		return nil, nil
	}
	curStart := dateutil.MonthStart(now)
	values := make([]SuggestionMonth, 0, months)
	for k := months; k >= 1; k-- {
		start := dateutil.AddMonths(curStart, -k)
		end := dateutil.AddMonths(curStart, -(k - 1))
		var amount int64
		for _, txn := range txns {
			if !txn.IsExpense() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, start, end) {
				continue
			}
			part, ok, err := scopedSpend(txn, covers, rates)
			if err != nil {
				return nil, err
			}
			if ok {
				amount, err = money.Add(amount, part)
				if err != nil {
					return nil, fmt.Errorf("sum budget spend for %s: %w", start.Format("2006-01"), err)
				}
			}
		}
		values = append(values, SuggestionMonth{Month: start.Format("2006-01"), Amount: amount})
	}
	// Ignore only the empty leading history before the category first had spend.
	first := 0
	for first < len(values) && values[first].Amount == 0 {
		first++
	}
	return values[first:], nil
}

func excludeSpendOutliers(values []SuggestionMonth) (used, excluded []SuggestionMonth, median int64, err error) {
	if len(values) == 0 {
		return nil, nil, 0, nil
	}
	amounts := make([]int64, len(values))
	for i := range values {
		amounts[i] = values[i].Amount
	}
	median, err = medianInt64(amounts)
	if err != nil {
		return nil, nil, 0, err
	}
	deviations := make([]int64, len(amounts))
	for i, amount := range amounts {
		if amount >= median {
			deviations[i] = amount - median
		} else {
			deviations[i] = median - amount
		}
	}
	mad, err := medianInt64(deviations)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, value := range values {
		deviation := value.Amount - median
		if deviation < 0 {
			deviation = -deviation
		}
		// A small floor keeps plausible consistent months when MAD is zero.
		floor, mulErr := money.MulDiv(median, 5, 100, money.RoundHalfAwayFromZero)
		if mulErr != nil {
			return nil, nil, 0, mulErr
		}
		if floor < 1 && median > 0 {
			floor = 1
		}
		if mad < floor {
			mad = floor
		}
		limit, mulErr := money.Mul(mad, 3)
		if mulErr != nil {
			return nil, nil, 0, mulErr
		}
		if deviation > limit {
			excluded = append(excluded, value)
		} else {
			used = append(used, value)
		}
	}
	if len(used) == 0 {
		return nil, excluded, median, nil
	}
	usedAmounts := make([]int64, len(used))
	for i := range used {
		usedAmounts[i] = used[i].Amount
	}
	median, err = medianInt64(usedAmounts)
	return used, excluded, median, err
}

func medianInt64(values []int64) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	ordered := append([]int64(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle], nil
	}
	sum, err := money.Add(ordered[middle-1], ordered[middle])
	if err != nil {
		return 0, err
	}
	return money.MulDiv(sum, 1, 2, money.RoundHalfAwayFromZero)
}
