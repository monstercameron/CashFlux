// SPDX-License-Identifier: MIT

package budgetview

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// IncomeSource is a selectable income-category fact. Minor is an independently
// truncated monthly average in the input rate table's base currency. It is only
// available when Known is true; known zero and unknown income remain distinct.
type IncomeSource struct {
	CategoryID string
	Name       string
	Minor      int64
	Known      bool
}

// IncomeSourceInput supplies explicit snapshots and the basis month's boundary.
// Months below one use one full trailing month. UncategorizedName is the current
// display label, which participates in the existing name tie-break ordering.
type IncomeSourceInput struct {
	Transactions      []domain.Transaction
	Categories        []domain.Category
	Rates             currency.Rates
	MonthStart        time.Time
	Months            int
	UncategorizedName string
}

// IncomeSourceModel keeps average availability independent of ranking facts.
// On amount failure, income-category metadata remains available for existing
// manual selections, but its rows are never represented as known zero history.
type IncomeSourceModel struct {
	Rows               []IncomeSource
	AmountErr, RankErr error
}

// PrepareIncomeSources builds source averages and stable last-month ranking
// using the canonical checked report reducer. Per-source truncation can leave a
// rounding remainder versus an average computed after combining all sources.
func PrepareIncomeSources(in IncomeSourceInput) IncomeSourceModel {
	months := max(1, in.Months)
	start := dateutil.AddMonths(in.MonthStart, -months)
	lines, amountErr := reports.IncomeByCategoryChecked(in.Transactions, start, in.MonthStart, in.Rates)
	rankLines, rankErr := lines, amountErr
	if months != 1 {
		rankLines, rankErr = reports.IncomeByCategoryChecked(in.Transactions, dateutil.AddMonths(in.MonthStart, -1), in.MonthStart, in.Rates)
	}
	out := IncomeSourceModel{Rows: make([]IncomeSource, 0, len(in.Categories)+1)}
	if amountErr != nil {
		out.AmountErr = fmt.Errorf("prepare average income sources: %w", amountErr)
	}
	if rankErr != nil {
		out.RankErr = fmt.Errorf("rank last-month income sources: %w", rankErr)
	}
	amounts := make(map[string]int64, len(lines))
	if amountErr == nil {
		for _, line := range lines {
			amounts[line.CategoryID] = line.Amount / int64(months)
		}
	}
	ranks := make(map[string]int64, len(rankLines))
	if rankErr == nil {
		for _, line := range rankLines {
			ranks[line.CategoryID] = line.Amount
		}
	}
	for _, category := range in.Categories {
		if category.Kind == domain.KindIncome {
			out.Rows = append(out.Rows, IncomeSource{CategoryID: category.ID, Name: category.Name, Minor: amounts[category.ID], Known: amountErr == nil})
		}
	}
	if amount := amounts[""]; amount > 0 {
		out.Rows = append(out.Rows, IncomeSource{Name: in.UncategorizedName, Minor: amount, Known: true})
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if left, right := ranks[out.Rows[i].CategoryID], ranks[out.Rows[j].CategoryID]; left != right {
			return left > right
		}
		return out.Rows[i].Name < out.Rows[j].Name
	})
	return out
}
