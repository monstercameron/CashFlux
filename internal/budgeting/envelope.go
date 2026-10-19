// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// EnvelopeAvailable returns the envelope balance available for the period
// containing curRef under envelope budgeting (D6). Each funded period uses its
// historical base and boost, starting at the later of the recorded start and
// first covered transaction. All covered contributions draw it down. The result is the
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
	in := ViewInput{Period: ViewPeriod{Anchor: curRef, Now: curRef, WeekStart: weekStart, PayCycleAnchor: anchor}, Transactions: all, Rates: rates, Base: rates.Base}
	view, err := EnvelopeRolloverView(budget, in, covers)
	if err != nil {
		return money.Money{}, err
	}
	return view.Current.EffectiveCap.Sub(view.Current.Spend)
}

// EnvelopeRolloverView uses the card's carry engine and historical bases. Funding
// starts at the later of the recorded start and first covered transaction period.
// An empty envelope, or a view before its first spend, funds one viewed period.
// Legacy rollover budgets share the card's one-prior-period compatibility start.
func EnvelopeRolloverView(b domain.Budget, in ViewInput, covers map[string]bool) (RolloverView, error) {
	p := in.Period
	current, _ := PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	first := current
	found := false
	ledger := NewNettedLedgerWithLinks(in.Transactions, nil)
	if in.Ledger != nil {
		ledger = *in.Ledger
	} else {
		ledger = NewNettedLedger(in.Transactions)
	}
	in.Ledger = &ledger
	for _, t := range ledger.transactions {
		_, ok, err := ContributionChecked(b, t, func(id string) bool { return covers[id] })
		if err != nil {
			return RolloverView{}, fmt.Errorf("envelope first contribution: %w", err)
		}
		if ok && (!found || t.Date.Before(first)) {
			first = t.Date
			found = true
		}
	}
	if found {
		first, _ = PeriodRangeAnchored(b.Period, first, p.WeekStart, p.PayCycleAnchor)
	}
	assumed := b.RolloverStart.IsZero() && b.Rollover
	start := b.RolloverStart
	if assumed {
		start, _ = PreviousPeriodRangeAnchored(b.Period, current, p.WeekStart, p.PayCycleAnchor)
	}
	if !start.IsZero() {
		start, _ = PeriodRangeAnchored(b.Period, start, p.WeekStart, p.PayCycleAnchor)
		if start.After(first) {
			first = start
		}
	}
	if first.After(current) {
		first = current
	}
	b.RolloverStart = first
	result, err := EvaluateRolloverView(b, in, covers)
	result.StartAssumed = assumed
	return result, err
}
