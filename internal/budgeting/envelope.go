// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// budgetCoversTxn reports whether the budget draws any spend from the
// transaction under pred, honoring per-line owners (XC10): for a split
// transaction, at least one line must pass pred and be owned by the budget's
// scope; for an atomic transaction, its category passes pred and its payer is in
// scope.
func budgetCoversTxn(budget domain.Budget, t domain.Transaction, pred func(string) bool) bool {
	if !t.CountsInReports() || !t.IsExpense() {
		return false
	}
	if budget.TracksAnyTag(t.Tags) {
		return ownsTransactionScope(budget, t)
	}
	if t.HasSplits() {
		for _, s := range t.Splits {
			if pred(s.CategoryID) && ownsSplitScope(budget, t, s) {
				return true
			}
		}
		return false
	}
	return pred(t.CategoryID) && ownsTransactionScope(budget, t)
}

// EnvelopeAvailable returns the envelope balance available for the period
// containing curRef under envelope budgeting (D6): every period from the first
// covered transaction through the current period is funded by the budget's
// limit, and all covered spend in that span draws it down. The result is the
// running balance carried forward — positive means money still in the envelope,
// negative means it's overdrawn. covers is the set of categories the budget
// counts (its own plus descendants, from categorytree.Descendants); an empty
// covers falls back to the budget's own category. Individual-scope budgets only
// count the owner's spend.
//
// With no covered spend yet, the envelope holds exactly one period's limit.
func EnvelopeAvailable(budget domain.Budget, all []domain.Transaction, curRef time.Time, weekStart time.Weekday, rates currency.Rates, covers map[string]bool) (money.Money, error) {
	return EnvelopeAvailableAnchored(budget, all, curRef, weekStart, time.Time{}, rates, covers)
}

// EnvelopeAvailableAnchored is EnvelopeAvailable with a pay-cycle anchor for
// biweekly budgets.
func EnvelopeAvailableAnchored(budget domain.Budget, all []domain.Transaction, curRef time.Time, weekStart time.Weekday, anchor time.Time, rates currency.Rates, covers map[string]bool) (money.Money, error) {
	limit := normalizedLimit(budget, rates)
	pred := func(id string) bool { return budget.TracksCategory(id) || covers[id] }
	ledger := NewNettedLedger(all)

	// Earliest covered, in-scope expense — the period funding starts from here.
	var earliest time.Time
	found := false
	for _, t := range all {
		// XC10: a split transaction is covered when any owned line matches; an
		// atomic transaction uses its own category and payer. Mirrors spentCovered
		// so the envelope's first-funded period lines up with what it draws down.
		if !budgetCoversTxn(budget, t, pred) {
			continue
		}
		if !found || t.Date.Before(earliest) {
			earliest, found = t.Date, true
		}
	}
	if !found {
		start, _ := PeriodRangeAnchored(budget.Period, curRef, weekStart, anchor)
		return limit.Add(money.New(budget.PeriodBoost(start), limit.Currency)) // funded once, nothing spent
	}

	curStart, _ := PeriodRangeAnchored(budget.Period, curRef, weekStart, anchor)
	periods := make([]RolloverPeriod, 0)
	ref := earliest
	for {
		ps, pe := PeriodRangeAnchored(budget.Period, ref, weekStart, anchor)
		spent, err := spentCoveredNetted(budget, ledger.transactions, ps, pe, rates, pred)
		if err != nil {
			return money.Money{}, err
		}
		base, err := limit.Add(money.New(budget.PeriodBoost(ps), limit.Currency))
		if err != nil {
			return money.Money{}, fmt.Errorf("period %s effective base: %w", ps.Format("2006-01-02"), err)
		}
		periods = append(periods, RolloverPeriod{Start: ps, Base: base, Spend: spent})
		if !ps.Before(curStart) {
			break // reached the current period (or beyond) — stop after funding it
		}
		if !pe.After(ps) {
			return money.Money{}, fmt.Errorf("envelope: period %s did not advance from %s", budget.Period, ps.Format("2006-01-02"))
		}
		ref = pe // step to the next period
	}
	chain, err := BuildRolloverChain(periods, budget.RolloverCapPeriods)
	if err != nil {
		return money.Money{}, fmt.Errorf("build rollover chain: %w", err)
	}
	if len(chain) == 0 {
		return money.Zero(limit.Currency), nil
	}
	return chain[len(chain)-1].EffectiveCap.Sub(chain[len(chain)-1].Spend)
}
