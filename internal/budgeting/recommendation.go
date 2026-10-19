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

func recommendFromComparableSpend(values []SuggestionMonth, currentLimit, currentProjection int64) (LimitRecommendation, error) {
	if len(values) == 0 {
		return LimitRecommendation{CurrentLimit: currentLimit, Suggested: currentLimit, Direction: SuggestionKeep, Projection: currentProjection}, nil
	}
	used, outliers, median, err := excludeSpendOutliers(values)
	if err != nil {
		return LimitRecommendation{}, err
	}
	recommended := median
	direction := SuggestionKeep
	switch {
	case recommended > currentLimit:
		// A raise needs either current-period overspend or repeated evidence in
		// at least two of the last three comparable periods. The projection is
		// a gate only; it never supplies the suggested amount.
		if currentProjection > currentLimit || overspentComparablePeriods(values, currentLimit) >= 2 {
			direction = SuggestionRaise
		} else {
			recommended = currentLimit
		}
	case recommended < currentLimit:
		direction = SuggestionLower
	}
	return LimitRecommendation{CurrentLimit: currentLimit, Suggested: recommended, Direction: direction, HasEvidence: true, Projection: currentProjection, Median: median, MonthsUsed: used, Outliers: outliers}, nil
}

func overspentComparablePeriods(values []SuggestionMonth, limit int64) int {
	start := len(values) - 3
	if start < 0 {
		start = 0
	}
	count := 0
	for _, value := range values[start:] {
		if value.Amount > limit {
			count++
		}
	}
	return count
}

// RecommendBudgetLimitWithContext evaluates a recommendation against the same
// budget scope and cadence grid used by the card. txns must be the caller's
// refund-netted ledger snapshot; weekStart and payAnchor define its period grid.
func RecommendBudgetLimitWithContext(budget domain.Budget, covers map[string]bool, txns NettedLedger, now time.Time, months int, projectedMonthly money.Money, rates currency.Rates, weekStart time.Weekday, payAnchor time.Time) (LimitRecommendation, error) {
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
	values, err := comparableBudgetSpend(budget, covers, txns, now, months, rates, weekStart, payAnchor)
	if err != nil {
		return LimitRecommendation{}, err
	}
	return recommendFromComparableSpend(values, monthly.Amount, projection.Amount)
}

func comparableBudgetSpend(budget domain.Budget, covers map[string]bool, txns NettedLedger, now time.Time, periods int, rates currency.Rates, weekStart time.Weekday, payAnchor time.Time) ([]SuggestionMonth, error) {
	if len(budget.TrackedCategoryIDs()) == 0 && len(covers) == 0 && len(budget.TrackedTags) == 0 || periods <= 0 {
		return nil, nil
	}
	start, _ := PeriodRangeAnchored(budget.Period, now, weekStart, payAnchor)
	values := make([]SuggestionMonth, 0, periods)
	for k := periods; k >= 1; k-- {
		windowEnd := start
		for i := 0; i < k-1; i++ {
			windowEnd, _ = PreviousPeriodRangeAnchored(budget.Period, windowEnd, weekStart, payAnchor)
		}
		windowStart, windowEnd := PreviousPeriodRangeAnchored(budget.Period, windowEnd, weekStart, payAnchor)
		var amount int64
		for _, txn := range txns {
			if !txn.IsExpense() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, windowStart, windowEnd) {
				continue
			}
			part, ok, err := ContributionChecked(budget, txn, func(id string) bool { return covers[id] })
			if err != nil {
				return nil, err
			}
			if ok {
				converted, convertErr := rates.Convert(part, rates.Base)
				if convertErr != nil {
					return nil, fmt.Errorf("convert budget contribution %q: %w", txn.ID, convertErr)
				}
				amount, err = money.Add(amount, converted.Amount)
				if err != nil {
					return nil, fmt.Errorf("sum budget spend for %s: %w", windowStart.Format("2006-01-02"), err)
				}
			}
		}
		monthly, err := PeriodEquivalent(money.New(amount, rates.Base), budget.Period, domain.PeriodMonthly)
		if err != nil {
			return nil, fmt.Errorf("normalize budget history for %s: %w", windowStart.Format("2006-01-02"), err)
		}
		values = append(values, SuggestionMonth{Month: windowStart.Format("2006-01-02"), Amount: monthly.Amount})
	}
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
