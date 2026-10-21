// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CategorySpendSeriesSelected shares the canonical expense/split fold while
// accumulating only selected category IDs. All eligible parent and negative
// split currencies and magnitudes are still checked. Unselected categories are
// checked within each transaction, preserving per-record validation without
// imposing an unused whole-window total. Nil or empty selection selects none.
// Caller-owned rows, splits and the selection map are never modified.
func CategorySpendSeriesSelected(all []domain.Transaction, bounds []time.Time, rates currency.Rates, selected map[string]bool) (map[string][]int64, error) {
	return categorySpendSeries(all, bounds, rates, selected, true)
}

func categorySpendSeries(all []domain.Transaction, bounds []time.Time, rates currency.Rates, selected map[string]bool, selectedOnly bool) (map[string][]int64, error) {
	n := len(bounds) - 1
	out := map[string][]int64{}
	if n < 1 {
		return out, nil
	}
	var unselectedTotals map[string]int64
	for _, t := range all {
		if !t.IsExpense() || !t.CountsInReports() {
			continue
		}
		idx := -1
		for i := 0; i < n; i++ {
			if dateutil.InRange(t.Date, bounds[i], bounds[i+1]) {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		if t.HasSplits() {
			clear(unselectedTotals)
			for _, split := range t.Splits {
				// Split lines carry their own direction. Positive lines are
				// refunds/credits and do not contribute to spend, even when the
				// parent transaction is an expense.
				if split.Amount.Amount >= 0 {
					continue
				}
				splitValue, splitErr := rates.Convert(split.Amount, rates.Base)
				if splitErr != nil {
					return nil, splitErr
				}
				value, splitErr := splitValue.AbsChecked()
				if splitErr != nil {
					return nil, fmt.Errorf("ledger: split spend: %w", splitErr)
				}
				if selectedOnly && !selected[split.CategoryID] {
					// Retain the original per-transaction category overflow
					// check without accumulating unused categories across rows.
					if unselectedTotals == nil {
						unselectedTotals = map[string]int64{}
					}
					next, addErr := money.Add(unselectedTotals[split.CategoryID], value.Amount)
					if addErr != nil {
						return nil, fmt.Errorf("ledger: category %q spend: %w", split.CategoryID, addErr)
					}
					unselectedTotals[split.CategoryID] = next
					continue
				}
				series := out[split.CategoryID]
				if series == nil {
					series = make([]int64, n)
					out[split.CategoryID] = series
				}
				next, addErr := money.Add(series[idx], value.Amount)
				if addErr != nil {
					return nil, fmt.Errorf("ledger: category %q spend: %w", split.CategoryID, addErr)
				}
				series[idx] = next
			}
			continue
		}
		value, err := conv.AbsChecked()
		if err != nil {
			return nil, fmt.Errorf("ledger: category %q spend: %w", t.CategoryID, err)
		}
		if selectedOnly && !selected[t.CategoryID] {
			continue
		}
		series, ok := out[t.CategoryID]
		if !ok {
			series = make([]int64, n)
			out[t.CategoryID] = series
		}
		next, err := money.Add(series[idx], value.Amount)
		if err != nil {
			return nil, fmt.Errorf("ledger: category %q spend: %w", t.CategoryID, err)
		}
		series[idx] = next
	}
	return out, nil
}
