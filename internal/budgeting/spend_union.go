// SPDX-License-Identifier: MIT

package budgeting

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SpendUnion is an immutable set of ordered budget scopes. It counts a tagged
// parent once, or each selected split line once across overlapping categories.
// ContributionChecked remains authoritative for ownership and money rules.
type SpendUnion struct{ scopes []spendUnionScope }

type spendUnionScope struct {
	budget domain.Budget
	covers map[string]bool
}

// PrepareSpendUnion snapshots the fields used by contribution matching and shares
// descendant sets for identical tracked roots. Callers select eligible budgets;
// the preparation does not read a store or retain mutable caller-owned slices.
func PrepareSpendUnion(categories []domain.Category, budgets []domain.Budget) SpendUnion {
	u := SpendUnion{scopes: make([]spendUnionScope, 0, len(budgets))}
	closures := make(map[string]map[string]bool)
	for _, b := range budgets {
		b.CategoryIDs = append([]string(nil), b.CategoryIDs...)
		b.TrackedTags = append([]string(nil), b.TrackedTags...)
		encoded, _ := json.Marshal(b.TrackedCategoryIDs()) // string slices always encode
		key := string(encoded)
		covers, ok := closures[key]
		if !ok {
			covers = categorytree.DescendantsOfAll(categories, b.TrackedCategoryIDs())
			closures[key] = covers
		}
		u.scopes = append(u.scopes, spendUnionScope{budget: b, covers: covers})
	}
	return u
}

// Contribution returns the selected signed spending in the transaction currency.
// It expects an already refund-netted transaction; dates and FX belong to the
// caller. Diagnostic order follows the input budget and split-line order.
func (u SpendUnion) Contribution(txn domain.Transaction) (money.Money, bool, []ViewIssue) {
	var issues []ViewIssue
	contribute := func(scope spendUnionScope, row domain.Transaction) (money.Money, bool) {
		amount, ok, err := ContributionChecked(scope.budget, row, func(id string) bool { return scope.covers[id] })
		if err != nil {
			issues = append(issues, ViewIssue{Operation: "evaluate selected-period budget spending", BudgetID: scope.budget.ID, Err: err})
			return money.Money{}, false
		}
		return amount, ok
	}
	for _, scope := range u.scopes {
		if scope.budget.TracksAnyTag(txn.Tags) {
			if amount, ok := contribute(scope, txn); ok {
				return amount, true, DeduplicateViewIssues(issues)
			}
		}
	}
	if !txn.HasSplits() {
		for _, scope := range u.scopes {
			if amount, ok := contribute(scope, txn); ok {
				return amount, true, DeduplicateViewIssues(issues)
			}
		}
		return money.Money{}, false, DeduplicateViewIssues(issues)
	}
	total := money.Zero(txn.Amount.Currency)
	matched := false
	for _, split := range txn.Splits {
		line := txn
		line.Splits = []domain.CategorySplit{split}
		for _, scope := range u.scopes {
			if amount, ok := contribute(scope, line); ok {
				var err error
				total, err = total.Add(amount)
				if err != nil {
					issues = append(issues, ViewIssue{Operation: "sum selected-period split spending", Err: err})
					return money.Money{}, false, DeduplicateViewIssues(issues)
				}
				matched = true
				break
			}
		}
	}
	return total, matched, DeduplicateViewIssues(issues)
}
