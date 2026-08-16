// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SuggestLimit proposes a monthly budget limit for a category from recent
// history: the average monthly spend in that category over the trailing `months`
// full months ending before the current month (the current partial month is
// excluded so it doesn't understate the figure). Amounts convert to the base
// currency; transfers and income are excluded.
//
// The average is taken over the span from the oldest month with spend through the
// most recent — so a brand-new category (one month of data) isn't diluted by
// empty leading months, while genuine zero-spend months within that span still
// count (they pull the average down, as a budget should reflect). Returns 0 when
// the category is empty, months is non-positive, or there's no spend to learn
// from.
func SuggestLimit(categoryID string, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if categoryID == "" {
		return 0, nil
	}
	return SuggestLimitIn(map[string]bool{categoryID: true}, txns, now, months, rates)
}

// SuggestLimitIn is SuggestLimit over a SET of categories — a budget's whole
// scope (its tracked categories plus their descendants) rather than one
// category id.
//
// The scoped form exists because the single-category form disagreed with the
// budget it was suggesting a limit for (C586). Copying a parent-category budget
// offered an average computed from charges booked directly to the parent, while
// the card beside it showed spend rolled up from every sub-category — two
// numbers for one budget, neither wrong on its own terms.
//
// It is also split-aware, as every category-side consumer must be (see the split
// contract in domain/category_split.go): a receipt split across categories
// contributes each line to its own category, never the whole amount to the
// transaction's category.
func SuggestLimitIn(covers map[string]bool, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if len(covers) == 0 || months <= 0 {
		return 0, nil
	}
	curStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	monthSums := make([]int64, months+1) // monthSums[k] = spend in the k-th prior full month
	for k := 1; k <= months; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		for _, t := range txns {
			if !t.IsExpense() || !dateutil.InRange(t.Date, ms, me) {
				continue
			}
			amt, ok, err := scopedSpend(t, covers, rates)
			if err != nil {
				return 0, err
			}
			if ok {
				monthSums[k] += amt
			}
		}
	}

	oldest := 0 // largest k that had any spend → span end
	var total int64
	for k := 1; k <= months; k++ {
		total += monthSums[k]
		if monthSums[k] > 0 {
			oldest = k
		}
	}
	if oldest == 0 {
		return 0, nil
	}
	return total / int64(oldest), nil
}

// scopedSpend returns the part of a transaction that falls inside covers, in the
// rate table's base currency. A split transaction contributes only its covered
// lines; an unsplit one contributes in full when its own category is covered.
func scopedSpend(t domain.Transaction, covers map[string]bool, rates currency.Rates) (int64, bool, error) {
	add := func(m money.Money) (int64, error) {
		conv, err := rates.Convert(m.Abs(), rates.Base)
		if err != nil {
			return 0, err
		}
		return conv.Amount, nil
	}
	if t.HasSplits() {
		var sum int64
		any := false
		for _, s := range t.Splits {
			if !covers[s.CategoryID] {
				continue
			}
			amt, err := add(s.Amount)
			if err != nil {
				return 0, false, err
			}
			sum += amt
			any = true
		}
		return sum, any, nil
	}
	if !covers[t.CategoryID] {
		return 0, false, nil
	}
	amt, err := add(t.Amount)
	if err != nil {
		return 0, false, err
	}
	return amt, true, nil
}
