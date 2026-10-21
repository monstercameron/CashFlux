// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// AdherenceState describes the report's published ten-percent tolerance above
// a spending cap. It is independent of the live budget page's pace warnings.
type AdherenceState string

const (
	// AdherenceQuiet means no spending was recorded.
	AdherenceQuiet AdherenceState = "quiet"
	// AdherenceWithin means spending did not exceed the cap.
	AdherenceWithin AdherenceState = "within"
	// AdherenceNear means spending exceeded the cap by at most ten percent.
	AdherenceNear AdherenceState = "near"
	// AdherenceOver means spending exceeded the cap by more than ten percent.
	AdherenceOver AdherenceState = "over"
)

// BudgetMonth holds one monthly adherence cell in base-currency minor units.
type BudgetMonth struct {
	Start time.Time
	Spent int64
	State AdherenceState
}

// BudgetAdherenceRow contains either monthly cells or a full-year comparison
// for a nonmonthly budget. Every monetary figure uses the display base currency.
type BudgetAdherenceRow struct {
	Budget                               domain.Budget
	Limit                                int64
	Months                               []BudgetMonth
	MonthsOver                           int
	AnnualLimit, AnnualSpent, AnnualOver int64
	AnnualPercent                        int64
	AnnualState                          AdherenceState
}

// BudgetAdherence is the report's complete evaluable budget rows and verdict
// counts. Failed rows never increase Clean or Counted.
type BudgetAdherence struct {
	Rows           []BudgetAdherenceRow
	Clean, Counted int
}

// BuildBudgetAdherence evaluates the report's monthly strips and explicitly
// annualized nonmonthly caps. all must be the household refund-netted ledger:
// these rows are labelled household-wide even when report activity is scoped.
// The existing review displays at most twelve monthly cells. An evaluation
// failure excludes the whole row and is returned alongside usable rows.
func BuildBudgetAdherence(budgets []domain.Budget, all budgeting.NettedLedger, bounds []time.Time, rates currency.Rates) (BudgetAdherence, error) {
	var out BudgetAdherence
	if len(bounds) < 2 {
		return out, nil
	}
	var failures []error
	for _, b := range budgets {
		row, err := buildBudgetAdherenceRow(b, all, bounds, rates)
		if err != nil {
			failures = append(failures, fmt.Errorf("report budget %q: %w", b.ID, err))
			continue
		}
		if row.Limit <= 0 {
			continue
		}
		out.Rows = append(out.Rows, row)
		out.Counted++
		if len(row.Months) > 0 && row.MonthsOver == 0 || len(row.Months) == 0 && row.AnnualSpent <= row.AnnualLimit {
			out.Clean++
		}
	}
	return out, errors.Join(failures...)
}

func buildBudgetAdherenceRow(b domain.Budget, all budgeting.NettedLedger, bounds []time.Time, rates currency.Rates) (BudgetAdherenceRow, error) {
	limit := b.Limit
	if limit.Currency == "" {
		limit = money.New(limit.Amount, rates.Base)
	}
	baseLimit, err := rates.Convert(limit, rates.Base)
	if err != nil {
		return BudgetAdherenceRow{}, err
	}
	row := BudgetAdherenceRow{Budget: b, Limit: baseLimit.Amount}
	if row.Limit <= 0 {
		return row, nil
	}
	spentBase := func(start, end time.Time) (int64, error) {
		spent, err := budgeting.Spent(b, all, start, end, rates)
		if err != nil {
			return 0, err
		}
		converted, err := rates.Convert(spent, rates.Base)
		return converted.Amount, err
	}
	if b.Period == domain.PeriodMonthly || b.Period == "" {
		for i := 0; i+1 < len(bounds) && i < 12; i++ {
			spent, err := spentBase(bounds[i], bounds[i+1])
			if err != nil {
				return BudgetAdherenceRow{}, err
			}
			state := adherenceState(spent, row.Limit)
			row.Months = append(row.Months, BudgetMonth{Start: bounds[i], Spent: spent, State: state})
			if state == AdherenceOver {
				row.MonthsOver++
			}
		}
		return row, nil
	}
	annual, err := budgeting.PeriodEquivalent(baseLimit, b.Period, domain.PeriodYearly)
	if err != nil {
		return BudgetAdherenceRow{}, err
	}
	row.AnnualLimit = annual.Amount
	row.AnnualSpent, err = spentBase(bounds[0], bounds[len(bounds)-1])
	if err != nil {
		return BudgetAdherenceRow{}, err
	}
	row.AnnualOver = max(row.AnnualSpent-row.AnnualLimit, 0)
	row.AnnualState = adherenceState(row.AnnualSpent, row.AnnualLimit)
	row.AnnualPercent, err = money.MulDiv(row.AnnualSpent, 100, row.AnnualLimit, money.RoundTowardZero)
	row.AnnualPercent = min(row.AnnualPercent, 100)
	return row, err
}

func adherenceState(spent, limit int64) AdherenceState {
	switch {
	case spent == 0:
		return AdherenceQuiet
	case spent <= limit:
		return AdherenceWithin
	case spent-limit <= limit/10:
		return AdherenceNear
	default:
		return AdherenceOver
	}
}
