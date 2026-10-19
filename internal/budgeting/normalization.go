// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// NormalizedBudget is one budget converted to a requested cadence and currency.
type NormalizedBudget struct {
	BudgetID string
	Name     string
	Period   domain.Period
	Original money.Money
	Amount   money.Money
}

// BudgetNormalization is an explainable sum of budgets on one cadence.
type BudgetNormalization struct {
	Period   domain.Period
	Currency string
	Lines    []NormalizedBudget
	Total    money.Money
}

// PeriodEquivalent converts a periodic amount using annual occurrence ratios,
// rounding once with CashFlux's half-away-from-zero policy.
func PeriodEquivalent(amount money.Money, from, to domain.Period) (money.Money, error) {
	fromCount, err := periodsPerYear(from)
	if err != nil {
		return money.Money{}, err
	}
	toCount, err := periodsPerYear(to)
	if err != nil {
		return money.Money{}, err
	}
	out, err := amount.MulDiv(int64(fromCount), int64(toCount), money.RoundHalfAwayFromZero)
	if err != nil {
		return money.Money{}, fmt.Errorf("normalize %s limit from %s to %s: %w", amount.Currency, from, to, err)
	}
	return out, nil
}

// NormalizeBudgets converts all limits to the requested period and rates.Base,
// returning each normalized line as well as the checked total.
func NormalizeBudgets(budgets []domain.Budget, period domain.Period, rates currency.Rates) (BudgetNormalization, error) {
	if rates.Base == "" {
		return BudgetNormalization{}, fmt.Errorf("normalize budgets: base currency is empty")
	}
	out := BudgetNormalization{Period: period, Currency: rates.Base, Lines: make([]NormalizedBudget, 0, len(budgets)), Total: money.Zero(rates.Base)}
	for _, budget := range budgets {
		original := budget.Limit
		if original.Currency == "" {
			original = money.New(original.Amount, rates.Base)
		}
		equivalent, err := PeriodEquivalent(original, budget.Period, period)
		if err != nil {
			return BudgetNormalization{}, fmt.Errorf("budget %q: %w", budget.ID, err)
		}
		converted, err := rates.Convert(equivalent, rates.Base)
		if err != nil {
			return BudgetNormalization{}, fmt.Errorf("budget %q currency conversion: %w", budget.ID, err)
		}
		out.Total, err = out.Total.Add(converted)
		if err != nil {
			return BudgetNormalization{}, fmt.Errorf("sum normalized budget %q: %w", budget.ID, err)
		}
		out.Lines = append(out.Lines, NormalizedBudget{BudgetID: budget.ID, Name: budget.Name, Period: budget.Period, Original: original, Amount: converted})
	}
	return out, nil
}

func periodsPerYear(period domain.Period) (int, error) {
	switch period {
	case domain.PeriodWeekly:
		return 52, nil
	case domain.PeriodBiweekly:
		return 26, nil
	case domain.PeriodSemimonthly:
		return 24, nil
	case domain.PeriodMonthly, "":
		return 12, nil
	case domain.PeriodQuarterly:
		return 4, nil
	case domain.PeriodYearly:
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported budget period %q", period)
	}
}
