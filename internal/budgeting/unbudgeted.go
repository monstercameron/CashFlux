// SPDX-License-Identifier: MIT

package budgeting

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// UnbudgetedCategory is one expense category with spend in the period that no budget
// tracks. Amount is the period expense in the budget base currency; Name is the display
// label (empty for uncategorized spend, which the UI renders as "Uncategorized").
type UnbudgetedCategory struct {
	CategoryID string
	Name       string
	Amount     money.Money
}

// Unbudgeted is the BG11 catch-all: every expense in [start, end) that falls in a category
// no budget covers, so /budgets can show a synthetic "Unbudgeted: $X this month" row that
// expands to the per-category breakdown. Total is the sum of Categories' amounts.
type Unbudgeted struct {
	Total      money.Money
	Categories []UnbudgetedCategory
}

// CoveredCategories returns the set of category ids covered by ANY budget's tracked tree —
// each budget's tracked categories plus their descendants (categorytree.DescendantsOfAll).
// It is the union used to decide what counts as "unbudgeted", and reusing the descendants
// machinery keeps sub-category tracking from being double-counted as a gap.
func CoveredCategories(budgets []domain.Budget, cats []domain.Category) map[string]bool {
	covered := make(map[string]bool)
	for _, b := range budgets {
		for id := range categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs()) {
			covered[id] = true
		}
	}
	return covered
}

// ComputeUnbudgeted sums reportable period expense in every category and tag NO budget
// covers, in base currency. It honours split lines, tracked tags, report exclusions, and
// XC2 refund netting. Overlapping budget scopes can mean budget totals overlap, so this
// catch-all is not guaranteed to partition total spending with budget rows. Uncategorized spend (empty CategoryID) is reported
// as one row with an empty CategoryID. Categories are sorted by amount, largest first, then
// by id; categories that net to zero or below are omitted.
func ComputeUnbudgeted(budgets []domain.Budget, cats []domain.Category, all []domain.Transaction, start, end time.Time, rates currency.Rates) (Unbudgeted, error) {
	covered := CoveredCategories(budgets, cats)
	nameByID := make(map[string]string, len(cats))
	for _, c := range cats {
		nameByID[c.ID] = c.Name
	}

	all = nettedForSpending(all)
	byCat := make(map[string]money.Money)
	add := func(categoryID string, amt money.Money) error {
		if covered[categoryID] {
			return nil
		}
		contribution, err := expenseContribution(amt)
		if err != nil {
			return err
		}
		conv, err := rates.Convert(contribution, rates.Base)
		if err != nil {
			return err
		}
		current := byCat[categoryID]
		if current.Currency == "" {
			current = money.Zero(rates.Base)
		}
		byCat[categoryID], err = current.Add(conv)
		return nil
	}

	for _, t := range all {
		if !t.CountsInReports() || !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		trackedTag := false
		for _, budget := range budgets {
			if budget.TracksAnyTag(t.Tags) {
				trackedTag = true
				break
			}
		}
		if trackedTag {
			continue
		}
		if t.HasSplits() {
			linesByCategory := make(map[string][]money.Money)
			for _, s := range t.Splits {
				linesByCategory[s.CategoryID] = append(linesByCategory[s.CategoryID], s.Amount)
			}
			for categoryID, lines := range linesByCategory {
				net, err := signedLineTotal(lines)
				if err != nil {
					return Unbudgeted{}, err
				}
				if err := add(categoryID, net); err != nil {
					return Unbudgeted{}, err
				}
			}
			continue
		}
		if err := add(t.CategoryID, t.Amount); err != nil {
			return Unbudgeted{}, err
		}
	}

	out := Unbudgeted{Total: money.Zero(rates.Base)}
	for id, amount := range byCat {
		if amount.Amount <= 0 {
			continue
		}
		out.Categories = append(out.Categories, UnbudgetedCategory{
			CategoryID: id, Name: nameByID[id], Amount: amount,
		})
		var err error
		out.Total, err = out.Total.Add(amount)
		if err != nil {
			return Unbudgeted{}, err
		}
	}
	sort.SliceStable(out.Categories, func(i, j int) bool {
		if out.Categories[i].Amount.Amount != out.Categories[j].Amount.Amount {
			return out.Categories[i].Amount.Amount > out.Categories[j].Amount.Amount
		}
		return out.Categories[i].CategoryID < out.Categories[j].CategoryID
	})
	return out, nil
}
