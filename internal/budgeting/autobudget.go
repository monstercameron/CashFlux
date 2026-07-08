// SPDX-License-Identifier: MIT

package budgeting

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SuggestMethod selects how a per-category monthly budget is learned from history.
type SuggestMethod string

const (
	// MethodRecent averages spend over the trailing window (SuggestLimit) — the quick
	// "budget my recent spending" take (the Smart, free tier).
	MethodRecent SuggestMethod = "recent"
	// MethodHealthy reviews a longer window and drops the single highest month before
	// averaging, so a one-off spike (a vacation, an annual bill) doesn't inflate the
	// target — a sustainable "healthy" average (the Smart+ tier).
	MethodHealthy SuggestMethod = "healthy"
)

// BudgetSuggestion is one category's proposed monthly budget, learned from spending
// history. MonthlyMinor is the suggested limit in base-currency minor units.
type BudgetSuggestion struct {
	CategoryID   string
	CategoryName string
	MonthlyMinor int64
}

// SuggestBudgets proposes a monthly budget for every EXPENSE category with learnable
// spend over the trailing `months` full months, using the chosen method. Categories
// with no spend to learn from are omitted. Results are sorted by suggested amount,
// largest first, so the biggest spending categories lead — the ones a budget matters
// most for.
//
// It is a pure function over the ledger: the UI layer decides which suggestions to
// pre-select and how the user tunes them before any budget is written.
func SuggestBudgets(categories []domain.Category, txns []domain.Transaction, now time.Time, months int, rates currency.Rates, method SuggestMethod) ([]BudgetSuggestion, error) {
	limit := SuggestLimit
	if method == MethodHealthy {
		limit = HealthyLimit
	}
	out := make([]BudgetSuggestion, 0, len(categories))
	for _, c := range categories {
		if c.Kind != domain.KindExpense {
			continue
		}
		minor, err := limit(c.ID, txns, now, months, rates)
		if err != nil {
			return nil, err
		}
		if minor <= 0 {
			continue
		}
		out = append(out, BudgetSuggestion{CategoryID: c.ID, CategoryName: c.Name, MonthlyMinor: minor})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].MonthlyMinor > out[j].MonthlyMinor })
	return out, nil
}

// HealthyLimit proposes a monthly budget like SuggestLimit but drops the single highest
// month within the learned span before averaging, so one blowout month (a holiday, an
// annual renewal) doesn't inflate an everyday target. With a one-month span it returns
// that month's spend (nothing to trim). Same currency/expense/partial-month rules as
// SuggestLimit. It is a func value so SuggestBudgets can dispatch between the two.
func HealthyLimit(categoryID string, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if categoryID == "" || months <= 0 {
		return 0, nil
	}
	curStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	monthSums := make([]int64, months+1)
	for k := 1; k <= months; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		for _, t := range txns {
			if !t.IsExpense() || t.CategoryID != categoryID || !dateutil.InRange(t.Date, ms, me) {
				continue
			}
			conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
			if err != nil {
				return 0, err
			}
			monthSums[k] += conv.Amount
		}
	}

	// Span = oldest month with spend .. most recent (k=1). Mirrors SuggestLimit so a
	// brand-new category isn't diluted by empty leading months.
	oldest := 0
	for k := 1; k <= months; k++ {
		if monthSums[k] > 0 {
			oldest = k
		}
	}
	if oldest == 0 {
		return 0, nil
	}
	span := monthSums[1 : oldest+1]
	if len(span) == 1 {
		return span[0], nil
	}
	// Drop the single highest month, then average the rest — the "healthy" trim.
	var total, hi int64
	for _, v := range span {
		total += v
		if v > hi {
			hi = v
		}
	}
	return (total - hi) / int64(len(span)-1), nil
}
