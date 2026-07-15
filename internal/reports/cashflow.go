// SPDX-License-Identifier: MIT

package reports

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// PeriodFlow is income, expense, and the derived net/savings for one reporting
// bucket, in base-currency minor units (both Income and Expense are non-negative;
// transfers are excluded). Start/End are the bucket's half-open bounds.
type PeriodFlow struct {
	Start   time.Time
	End     time.Time
	Income  int64
	Expense int64
}

// Net is income minus expense (positive = saved, negative = overspent).
func (f PeriodFlow) Net() int64 { return f.Income - f.Expense }

// SavingsRate is the percent of income kept (net / income), 0 when there was no
// income, via the shared ledger rule so it matches the dashboard KPI.
func (f PeriodFlow) SavingsRate() int { return ledger.SavingsRate(f.Income, f.Expense) }

// IncomeVsExpense totals income and expense over a single half-open period
// [start, end) in the base currency (transfers excluded), reusing
// ledger.PeriodTotals so it matches the rest of the app.
func IncomeVsExpense(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (PeriodFlow, error) {
	txns = netted(txns) // XC2: fold refund-pair netting into period totals
	inc, exp, err := ledger.PeriodTotals(txns, start, end, rates)
	if err != nil {
		return PeriodFlow{}, err
	}
	return PeriodFlow{Start: start, End: end, Income: inc.Amount, Expense: exp.Amount}, nil
}

// IncomeExpenseSeries totals income and expense for each consecutive bucket
// defined by bounds — bucket i is [bounds[i], bounds[i+1]) — oldest first, for
// the cash-flow trend chart. It returns exactly len(bounds)-1 flows; fewer than
// two bounds yields an empty result.
func IncomeExpenseSeries(txns []domain.Transaction, bounds []time.Time, rates currency.Rates) ([]PeriodFlow, error) {
	n := len(bounds) - 1
	if n < 1 {
		return nil, nil
	}
	out := make([]PeriodFlow, 0, n)
	for i := 0; i < n; i++ {
		f, err := IncomeVsExpense(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// TrailingMonthlyNet returns the average monthly net cash flow (income minus
// expense, base currency minor units) over the `months` COMPLETE calendar months
// immediately before the month containing `now` — a steadier basis for a forecast
// than a single (possibly atypical) current month (L27). months <= 0 falls back to
// 1. Transfers are excluded via PeriodTotals.
func TrailingMonthlyNet(txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if months <= 0 {
		months = 1
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var sum int64
	for i := 1; i <= months; i++ {
		s := dateutil.AddMonths(monthStart, -i)
		e := dateutil.AddMonths(monthStart, -i+1)
		income, expense, err := ledger.PeriodTotals(txns, s, e, rates)
		if err != nil {
			return 0, err
		}
		sum += income.Amount - expense.Amount
	}
	return sum / int64(months), nil
}
