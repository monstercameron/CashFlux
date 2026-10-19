// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Contribution returns the signed expense (or saving contribution) selected by
// a budget, in the transaction currency. It honours report exclusions, tags,
// direction and per-line ownership. covers adds descendants to tracked categories.
// Dates and refund netting belong to the caller's immutable ledger/window.
// Invalid money returns false; calculations must use ContributionChecked to
// distinguish invalid data from an out-of-scope transaction.
func Contribution(b domain.Budget, t domain.Transaction, covers func(string) bool) (money.Money, bool) {
	amount, ok, err := ContributionChecked(b, t, covers)
	return amount, ok && err == nil
}

// ContributionChecked is Contribution with explicit overflow and currency errors.
func ContributionChecked(b domain.Budget, t domain.Transaction, covers func(string) bool) (money.Money, bool, error) {
	if !t.CountsInReports() || !inScopeDirection(b, t) {
		return money.Money{}, false, nil
	}
	tracked := func(id string) bool { return b.TracksCategory(id) || covers != nil && covers(id) }
	signed := t.Amount
	if b.TracksAnyTag(t.Tags) {
		if !ownsTransactionScope(b, t) {
			return money.Money{}, false, nil
		}
	} else if t.HasSplits() {
		var lines []money.Money
		for _, s := range t.Splits {
			if tracked(s.CategoryID) && ownsSplitScope(b, t, s) {
				lines = append(lines, s.Amount)
			}
		}
		if len(lines) == 0 {
			return money.Money{}, false, nil
		}
		var err error
		signed, err = signedLineTotal(lines)
		if err != nil {
			return money.Money{}, false, fmt.Errorf("transaction %q contribution: %w", t.ID, err)
		}
	} else if !tracked(t.CategoryID) || !ownsTransactionScope(b, t) {
		return money.Money{}, false, nil
	}
	amount, err := expenseContribution(signed)
	if err != nil {
		return money.Money{}, false, fmt.Errorf("transaction %q contribution: %w", t.ID, err)
	}
	return amount, true, nil
}
