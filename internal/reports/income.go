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
		if !t.IsIncome() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
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
		if !t.IsIncome() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		// C533: attribute a SPLIT deposit per line, so a paycheck split into
		// salary + bonus shows under both sources rather than landing wholly on
		// the transaction's own category. Any amount the lines do not cover stays
		// on the transaction's category, so the rows still sum to the deposit and
		// this total keeps agreeing with budgeting.ZeroBasedIncome.
		if t.HasSplits() {
			var allocated int64
			for _, s := range t.Splits {
				conv, err := rates.Convert(s.Amount, rates.Base)
				if err != nil {
					return nil, err
				}
				totals[s.CategoryID] += conv.Abs().Amount
				allocated += conv.Abs().Amount
			}
			whole, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				return nil, err
			}
			if rest := whole.Abs().Amount - allocated; rest > 0 {
				totals[t.CategoryID] += rest
			}
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
