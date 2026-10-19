// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"sort"
	"time"
)

// CompositionSlice is a signed category contribution in the budget currency.
// Discounts stay negative; callers must retain them when presenting composition.
type CompositionSlice struct {
	CategoryID string
	Amount     money.Money
}

// ContributingTransactions returns the selected amount of each covered
// transaction for goal-funded attribution. Splits are resolved by Contribution;
// the returned copies have no splits, so later attribution cannot recount them.
// Refund links must be resolved on the input ledger before calling this helper.
func ContributingTransactions(b domain.Budget, all NettedLedger, start, end time.Time, covers map[string]bool) ([]domain.Transaction, error) {
	var out []domain.Transaction
	for _, t := range all {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		amount, ok, err := ContributionChecked(b, t, func(id string) bool { return covers[id] })
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		signed, err := amount.NegChecked()
		if err != nil {
			return nil, err
		}
		t.Amount = signed
		t.Splits = nil
		out = append(out, t)
	}
	return out, nil
}

// CompositionSlices uses the same contribution and refund rules as SpentRollup.
// Each transaction is converted once for its total; any category rounding residue
// goes to its last sorted category. Consequently the signed slices sum exactly
// to Spent, including discounts, tags, descendants, FX and individual ownership.
func CompositionSlices(b domain.Budget, all NettedLedger, start, end time.Time, rates currency.Rates, covers map[string]bool) ([]CompositionSlice, error) {
	totals := map[string]money.Money{}
	cur := normalizedLimit(b, rates).Currency
	for _, t := range all {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		amount, ok, err := ContributionChecked(b, t, func(id string) bool { return covers[id] })
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		whole, err := rates.Convert(amount, cur)
		if err != nil {
			return nil, err
		}
		parts := map[string]money.Money{}
		if b.TracksAnyTag(t.Tags) || !t.HasSplits() {
			parts[t.CategoryID] = amount
		} else {
			ids := map[string]bool{}
			for _, s := range t.Splits {
				if b.TracksCategory(s.CategoryID) || covers[s.CategoryID] {
					ids[s.CategoryID] = true
				}
			}
			scoped := b
			scoped.CategoryID = ""
			scoped.CategoryIDs = nil
			scoped.TrackedTags = nil
			for id := range ids {
				part, ok, err := ContributionChecked(scoped, t, func(category string) bool { return category == id })
				if err != nil {
					return nil, err
				}
				if ok {
					parts[id] = part
				}
			}
		}
		ids := make([]string, 0, len(parts))
		for id := range parts {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		left := whole
		for i, id := range ids {
			converted := left
			if i < len(ids)-1 {
				converted, err = rates.Convert(parts[id], cur)
				if err != nil {
					return nil, err
				}
			}
			left, err = left.Sub(converted)
			if err != nil {
				return nil, fmt.Errorf("composition remainder: %w", err)
			}
			total := totals[id]
			if total.Currency == "" {
				total = money.Zero(cur)
			}
			totals[id], err = total.Add(converted)
			if err != nil {
				return nil, fmt.Errorf("composition category %q: %w", id, err)
			}
		}
	}
	ids := make([]string, 0, len(totals))
	for id := range totals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]CompositionSlice, 0, len(ids))
	for _, id := range ids {
		out = append(out, CompositionSlice{CategoryID: id, Amount: totals[id]})
	}
	return out, nil
}
