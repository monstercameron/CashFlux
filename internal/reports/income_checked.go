// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// IncomeByCategoryChecked applies the same per-category split and remainder
// policy as IncomeByCategory while preserving conversion, magnitude, and sum
// errors instead of returning wrapped totals.
func IncomeByCategoryChecked(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([]CategorySpend, error) {
	totals := map[string]int64{}
	add := func(categoryID string, amount int64) error {
		next, err := money.Add(totals[categoryID], amount)
		if err != nil {
			return err
		}
		totals[categoryID] = next
		return nil
	}
	for _, t := range txns {
		if !t.IsIncome() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if t.HasSplits() {
			var allocated int64
			for _, split := range t.Splits {
				converted, err := rates.Convert(split.Amount, rates.Base)
				if err != nil {
					return nil, err
				}
				amount, err := converted.AbsChecked()
				if err != nil {
					return nil, err
				}
				if err := add(split.CategoryID, amount.Amount); err != nil {
					return nil, err
				}
				allocated, err = money.Add(allocated, amount.Amount)
				if err != nil {
					return nil, err
				}
			}
			whole, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				return nil, err
			}
			whole, err = whole.AbsChecked()
			if err != nil {
				return nil, err
			}
			remainder, err := money.Sub(whole.Amount, allocated)
			if err != nil {
				return nil, err
			}
			if remainder > 0 {
				if err := add(t.CategoryID, remainder); err != nil {
					return nil, err
				}
			}
			continue
		}
		converted, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		amount, err := converted.AbsChecked()
		if err != nil {
			return nil, err
		}
		if err := add(t.CategoryID, amount.Amount); err != nil {
			return nil, err
		}
	}

	rows := make([]CategorySpend, 0, len(totals))
	for id, amount := range totals {
		rows = append(rows, CategorySpend{CategoryID: id, Amount: amount})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Amount != rows[j].Amount {
			return rows[i].Amount > rows[j].Amount
		}
		return rows[i].CategoryID < rows[j].CategoryID
	})
	return rows, nil
}
