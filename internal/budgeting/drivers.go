// SPDX-License-Identifier: MIT

package budgeting

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Driver is one charge (a whole transaction, or a single split line) counting
// toward a budget's spend — the raw material for a "what's driving this?" panel
// that answers WHY a budget is over, not just that it is.
type Driver struct {
	TxnID  string
	Label  string   // payee if set, else the transaction description
	Amount int64    // magnitude in the budget's limit currency (always positive)
	Date   time.Time
	Tags   []string // the transaction's tags (so the UI can spot a subscription/recurring driver)
}

// TopDrivers returns up to n of the largest charges counting toward the budget over
// [start, end), sorted largest-first. It applies exactly the same matching as Spent
// (tracked-tag priority counts a charge once and whole; split lines attribute to the
// covered category and their line owner; individual budgets only count what they own),
// so the drivers always reconcile with the Spent figure. `covers` is the category
// predicate — pass a rollup descendant set (EvaluateRollup) or budget.TracksCategory.
func TopDrivers(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, covers func(string) bool, n int) ([]Driver, error) {
	if n <= 0 {
		return nil, nil
	}
	limit := normalizedLimit(budget, rates)
	all = nettedForSpending(all)
	label := func(t domain.Transaction) string {
		if s := strings.TrimSpace(t.Payee); s != "" {
			return s
		}
		return strings.TrimSpace(t.Desc)
	}
	var drivers []Driver
	push := func(t domain.Transaction, amt money.Money) error {
		conv, err := rates.Convert(amt.Abs(), limit.Currency)
		if err != nil {
			return err
		}
		if conv.Amount <= 0 {
			return nil
		}
		drivers = append(drivers, Driver{TxnID: t.ID, Label: label(t), Amount: conv.Amount, Date: t.Date, Tags: t.Tags})
		return nil
	}
	for _, t := range all {
		if !t.CountsInReports() || !matchesScope(budget, t, start, end) {
			continue
		}
		if budget.TracksAnyTag(t.Tags) {
			if !ownsScope(budget, t.MemberID) {
				continue
			}
			if err := push(t, t.Amount); err != nil {
				return nil, err
			}
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if !covers(s.CategoryID) || !ownsScope(budget, s.LineOwner(t.MemberID)) {
					continue
				}
				if err := push(t, s.Amount); err != nil {
					return nil, err
				}
			}
			continue
		}
		if !ownsScope(budget, t.MemberID) || !covers(t.CategoryID) {
			continue
		}
		if err := push(t, t.Amount); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(drivers, func(i, j int) bool { return drivers[i].Amount > drivers[j].Amount })
	if len(drivers) > n {
		drivers = drivers[:n]
	}
	return drivers, nil
}
