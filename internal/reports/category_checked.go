// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SpendingByCategoryChecked reads an explicitly prepared refund ledger without
// consulting the process-wide legacy refund adapter. It uses the canonical
// checked category-series fold: report exclusions and transfers are omitted,
// negative split lines belong to their categories, and positive credit lines do
// not become additional spending. The window is half open [start,end).
func SpendingByCategoryChecked(all budgeting.NettedLedger, start, end time.Time, rates currency.Rates) ([]CategorySpend, error) {
	series, err := ledger.CategorySpendSeries(all, []time.Time{start, end}, rates)
	if err != nil {
		return nil, err
	}
	rows := make([]CategorySpend, 0, len(series))
	for category, values := range series {
		rows = append(rows, CategorySpend{CategoryID: category, Amount: values[0]})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Amount != rows[j].Amount {
			return rows[i].Amount > rows[j].Amount
		}
		return rows[i].CategoryID < rows[j].CategoryID
	})
	return rows, nil
}

// TotalChecked sums already prepared category rows without wrapping a total.
func TotalChecked(rows []CategorySpend) (int64, error) {
	var total int64
	for _, row := range rows {
		var err error
		total, err = money.Add(total, row.Amount)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// CategoryComparisonChecked prepares the same current/prior category union and
// new-category flags as SpendingByCategory from an explicit refund ledger.
func CategoryComparisonChecked(all budgeting.NettedLedger, start, end time.Time, compare bool, priorStart, priorEnd time.Time, rates currency.Rates) ([]CategorySpend, error) {
	current, err := SpendingByCategoryChecked(all, start, end, rates)
	if err != nil {
		return nil, err
	}
	if !compare {
		return current, nil
	}
	previous, err := SpendingByCategoryChecked(all, priorStart, priorEnd, rates)
	if err != nil {
		return nil, err
	}
	byID := map[string]CategorySpend{}
	for _, row := range current {
		byID[row.CategoryID] = row
	}
	for _, row := range previous {
		v := byID[row.CategoryID]
		v.CategoryID, v.Prior = row.CategoryID, row.Amount
		byID[row.CategoryID] = v
	}
	rows := make([]CategorySpend, 0, len(byID))
	for _, row := range byID {
		row.PriorZero = row.Prior == 0 && row.Amount > 0
		if row.Prior != 0 {
			delta, err := money.Sub(row.Amount, row.Prior)
			if err != nil {
				return nil, err
			}
			row.DeltaPct, err = money.MulDiv(delta, 100, row.Prior, money.RoundTowardZero)
			if err != nil {
				return nil, fmt.Errorf("category %q change: %w", row.CategoryID, err)
			}
			row.HasDelta = true
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Amount != rows[j].Amount {
			return rows[i].Amount > rows[j].Amount
		}
		return rows[i].CategoryID < rows[j].CategoryID
	})
	return rows, nil
}

// CategoryTrendsChecked derives exact-bucket trends from the shared checked
// ledger fold, preserving magnitude, accumulated total and percentage errors.
func CategoryTrendsChecked(all budgeting.NettedLedger, bounds []time.Time, rates currency.Rates) ([]CategoryTrend, error) {
	series, err := ledger.CategorySpendSeries(all, bounds, rates)
	if err != nil {
		return nil, err
	}
	rows := make([]CategoryTrend, 0, len(series))
	for id, spend := range series {
		row := CategoryTrend{CategoryID: id, Spend: spend}
		for _, v := range spend {
			row.Total, err = money.Add(row.Total, v)
			if err != nil {
				return nil, err
			}
		}
		if len(spend) > 0 && spend[0] != 0 {
			delta, err := money.Sub(spend[len(spend)-1], spend[0])
			if err != nil {
				return nil, err
			}
			row.DeltaPct, err = money.MulDiv(delta, 100, spend[0], money.RoundTowardZero)
			if err != nil {
				return nil, err
			}
			row.HasDelta = true
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].CategoryID < rows[j].CategoryID
	})
	return rows, nil
}
