// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// LargestIncome returns the biggest individual income transactions over the
// half-open period [start, end), largest first — "where the biggest deposits
// came from", the income mirror of LargestExpenses. Only income (positive,
// non-transfer) counts; amounts convert to the base currency. It reuses
// ExpenseItem as the generic largest-item shape. n <= 0 returns them all; ties
// break by most recent date, then description.
func LargestIncome(txns []domain.Transaction, start, end time.Time, rates currency.Rates, n int) ([]ExpenseItem, error) {
	var out []ExpenseItem
	for _, t := range txns {
		if !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpenseItem{Desc: t.Desc, CategoryID: t.CategoryID, Amount: conv.Abs().Amount, Date: t.Date})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.After(out[j].Date)
		}
		return out[i].Desc < out[j].Desc
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// IncomeByCategory totals income by category over the half-open period
// [start, end) in the base currency, largest first (ties broken by category id
// for determinism) — the "where the money comes from" view, mirroring
// SpendingByCategory. Only income transactions count (positive, non-transfer);
// expenses and transfers are excluded. CategoryID is empty for uncategorized
// income; the caller resolves names. The result reuses CategorySpend with only
// the Amount field populated (no prior-period comparison).
func IncomeByCategory(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([]CategorySpend, error) {
	totals := map[string]int64{}
	for _, t := range txns {
		if !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		totals[t.CategoryID] += conv.Abs().Amount
	}

	out := make([]CategorySpend, 0, len(totals))
	for id, amt := range totals {
		out = append(out, CategorySpend{CategoryID: id, Amount: amt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}
