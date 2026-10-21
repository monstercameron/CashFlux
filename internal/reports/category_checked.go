// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/ledger"
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
