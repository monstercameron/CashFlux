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

// Driver is one MERCHANT's total spend counting toward a budget over the period —
// the raw material for a "what's driving this?" panel that answers WHY a budget is
// over, not just that it is. Charges are aggregated per merchant (see TopDrivers)
// so a store that appears under several raw descriptors, or racks up many small
// charges, surfaces as the single line a user means by "what's driving this".
type Driver struct {
	TxnID  string   // the merchant's most recent contributing transaction (for reference)
	Label  string   // the merchant's normalized display name
	Amount int64    // summed magnitude in the budget's limit currency (always positive)
	Date   time.Time // the most recent contributing charge
	Tags   []string // union of the contributing charges' tags (so the UI can spot a recurring driver)
}

// TopDrivers returns up to n of the largest MERCHANTS counting toward the budget
// over [start, end), sorted largest-first. It applies exactly the same matching as
// Spent (tracked-tag priority counts a charge once and whole; split lines attribute
// to the covered category and their line owner; individual budgets only count what
// they own), so the drivers reconcile with the Spent figure. Charges are grouped by
// merchant: `normalize` maps a raw payee/description to a clean display name (pass
// the payee-alias resolver, or nil for a plain trim) and the grouping is by that
// name, case-insensitively — so "Greenfield Market" and "APLPAY GREENFIELD MKT #204"
// collapse to one line, and ten small coffee runs rank as their true combined total.
// `covers` is the category predicate — pass a rollup descendant set (EvaluateRollup)
// or budget.TracksCategory.
func TopDrivers(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, covers func(string) bool, n int, normalize func(string) string) ([]Driver, error) {
	if n <= 0 {
		return nil, nil
	}
	limit := normalizedLimit(budget, rates)
	all = nettedForSpending(all)
	rawLabel := func(t domain.Transaction) string {
		if s := strings.TrimSpace(t.Payee); s != "" {
			return s
		}
		return strings.TrimSpace(t.Desc)
	}
	display := func(raw string) string {
		if normalize != nil {
			if d := strings.TrimSpace(normalize(raw)); d != "" {
				return d
			}
		}
		return raw
	}

	type agg struct {
		label  string
		amount int64
		date   time.Time
		txnID  string
		tags   map[string]bool
	}
	byKey := map[string]*agg{}
	var order []string // first-seen order, for deterministic tie-breaks
	push := func(t domain.Transaction, amt money.Money) error {
		conv, err := rates.Convert(amt.Abs(), limit.Currency)
		if err != nil {
			return err
		}
		if conv.Amount <= 0 {
			return nil
		}
		disp := display(rawLabel(t))
		key := strings.ToLower(disp)
		a := byKey[key]
		if a == nil {
			a = &agg{label: disp, date: t.Date, txnID: t.ID, tags: map[string]bool{}}
			byKey[key] = a
			order = append(order, key)
		}
		a.amount += conv.Amount
		if t.Date.After(a.date) {
			a.date = t.Date
			a.txnID = t.ID
		}
		for _, tg := range t.Tags {
			a.tags[tg] = true
		}
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

	drivers := make([]Driver, 0, len(order))
	for _, key := range order {
		a := byKey[key]
		tags := make([]string, 0, len(a.tags))
		for tg := range a.tags {
			tags = append(tags, tg)
		}
		sort.Strings(tags)
		drivers = append(drivers, Driver{TxnID: a.txnID, Label: a.label, Amount: a.amount, Date: a.date, Tags: tags})
	}
	// Largest first; ties broken by label so the order is deterministic despite the
	// map grouping.
	sort.SliceStable(drivers, func(i, j int) bool {
		if drivers[i].Amount != drivers[j].Amount {
			return drivers[i].Amount > drivers[j].Amount
		}
		return drivers[i].Label < drivers[j].Label
	})
	if len(drivers) > n {
		drivers = drivers[:n]
	}
	return drivers, nil
}
