// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// AnnualGridCell is one budget's plan vs actual for one calendar month (BG9).
type AnnualGridCell struct {
	// Plan is the budget's monthly-equivalent limit for the month (major-unit money
	// in the budget's limit currency).
	Plan money.Money
	// Actual is spend against the budget over that calendar month (rollup-aware).
	Actual money.Money
	// Over reports whether Actual exceeded Plan for the month — the toned cell.
	Over bool
}

// AnnualGridRow is one budget's twelve-month plan-vs-actual strip plus its totals.
type AnnualGridRow struct {
	BudgetID    string
	Name        string
	Cells       [12]AnnualGridCell // index 0 = January … 11 = December
	PlanTotal   money.Money        // Σ of the row's monthly plans
	ActualTotal money.Money        // Σ of the row's monthly actuals
}

// AnnualGrid is the categories×months plan-vs-actual matrix for a year (BG9): a
// pure projection of the per-month budget evaluations the engine already computes.
// It is view-only — a reviewable year narrative with row and column totals, a
// highlighted current month, and over-toned cells.
type AnnualGrid struct {
	Year              int
	Currency          string
	Rows              []AnnualGridRow
	MonthPlanTotals   [12]money.Money // column plan totals across all budgets
	MonthActualTotals [12]money.Money // column actual totals across all budgets
	GrandPlan         money.Money
	GrandActual       money.Money
	// CurrentMonth is the 0-based month index (0 = January) to highlight when the
	// grid's year is the current year, else -1 (no highlight for a past/future year).
	CurrentMonth int
}

// monthlyLimitEquivalent normalizes a budget's per-period limit to a monthly
// figure so a weekly or quarterly budget's plan compares on the same scale as a
// monthly one in the annual grid. It rounds to the nearest minor unit.
func monthlyLimitEquivalent(period domain.Period, limit money.Money) money.Money {
	amt := limit.Amount
	var scaled int64
	switch period {
	case domain.PeriodWeekly:
		scaled = (amt*52 + 6) / 12
	case domain.PeriodBiweekly:
		scaled = (amt*26 + 6) / 12
	case domain.PeriodSemimonthly:
		scaled = amt * 2
	case domain.PeriodQuarterly:
		scaled = (amt + 1) / 3
	case domain.PeriodYearly:
		scaled = (amt + 6) / 12
	default: // monthly (and any unknown period falls back to monthly)
		scaled = amt
	}
	return money.New(scaled, limit.Currency)
}

// BuildAnnualGrid projects every budget's plan vs actual across the twelve
// calendar months of year (BG9). Actual spend per cell is the rollup-aware
// EvaluateRollup spend over that calendar month; covers[budgetID] supplies the
// sub-category rollup set for a budget (nil → the budget's own categories). The
// plan per cell is the budget's monthly-equivalent limit. Row, column, and grand
// totals are accumulated in the shared grid currency (the rate table's base).
func BuildAnnualGrid(budgets []domain.Budget, all []domain.Transaction, year int, rates currency.Rates, weekStart time.Weekday, now time.Time, covers map[string]map[string]bool) (AnnualGrid, error) {
	cur := rates.Base
	if cur == "" {
		cur = "USD"
	}
	grid := AnnualGrid{Year: year, Currency: cur, CurrentMonth: -1}
	if now.Year() == year {
		grid.CurrentMonth = int(now.Month()) - 1
	}
	for i := 0; i < 12; i++ {
		grid.MonthPlanTotals[i] = money.Zero(cur)
		grid.MonthActualTotals[i] = money.Zero(cur)
	}
	grid.GrandPlan = money.Zero(cur)
	grid.GrandActual = money.Zero(cur)

	for _, b := range budgets {
		row := AnnualGridRow{BudgetID: b.ID, Name: b.Name}
		row.PlanTotal = money.Zero(cur)
		row.ActualTotal = money.Zero(cur)
		plan := monthlyLimitEquivalent(b.Period, normalizedLimit(b, rates))
		cov := covers[b.ID]
		for m := 0; m < 12; m++ {
			start := time.Date(year, time.Month(m+1), 1, 0, 0, 0, 0, now.Location())
			end := time.Date(year, time.Month(m+2), 1, 0, 0, 0, 0, now.Location())
			st, err := EvaluateRollup(b, all, start, end, rates, DefaultNearThreshold, cov)
			if err != nil {
				return AnnualGrid{}, err
			}
			// Convert the plan to the grid currency so mixed-currency budgets sum.
			planBase, err := rates.Convert(plan, cur)
			if err != nil {
				return AnnualGrid{}, err
			}
			actualBase, err := rates.Convert(st.Spent, cur)
			if err != nil {
				return AnnualGrid{}, err
			}
			cell := AnnualGridCell{
				Plan:   planBase,
				Actual: actualBase,
				Over:   actualBase.Amount > planBase.Amount,
			}
			row.Cells[m] = cell
			if row.PlanTotal, err = row.PlanTotal.Add(planBase); err != nil {
				return AnnualGrid{}, err
			}
			if row.ActualTotal, err = row.ActualTotal.Add(actualBase); err != nil {
				return AnnualGrid{}, err
			}
			if grid.MonthPlanTotals[m], err = grid.MonthPlanTotals[m].Add(planBase); err != nil {
				return AnnualGrid{}, err
			}
			if grid.MonthActualTotals[m], err = grid.MonthActualTotals[m].Add(actualBase); err != nil {
				return AnnualGrid{}, err
			}
		}
		var err error
		if grid.GrandPlan, err = grid.GrandPlan.Add(row.PlanTotal); err != nil {
			return AnnualGrid{}, err
		}
		if grid.GrandActual, err = grid.GrandActual.Add(row.ActualTotal); err != nil {
			return AnnualGrid{}, err
		}
		grid.Rows = append(grid.Rows, row)
	}
	return grid, nil
}
