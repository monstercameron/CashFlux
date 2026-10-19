// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// EnvelopeAvailableAnchored returns the available balance for the period containing curRef, using the shared rollover view and pay-cycle anchor.
func EnvelopeAvailableAnchored(budget domain.Budget, all []domain.Transaction, curRef time.Time, weekStart time.Weekday, anchor time.Time, rates currency.Rates, covers map[string]bool) (money.Money, error) {
	in := ViewInput{Period: ViewPeriod{Anchor: curRef, Now: curRef, WeekStart: weekStart, PayCycleAnchor: anchor}, Transactions: all, Rates: rates, Base: rates.Base}
	view, err := EnvelopeRolloverView(budget, in, covers)
	if err != nil {
		return money.Money{}, err
	}
	return view.Current.EffectiveCap.Sub(view.Current.Spend)
}

// EnvelopeRolloverView uses the card carry engine and historical bases. Rollover budgets use their recorded or compatibility start; envelope-only budgets start with their first covered contribution and fund one period when empty.
func EnvelopeRolloverView(b domain.Budget, in ViewInput, covers map[string]bool) (RolloverView, error) {
	if b.Rollover {
		return EvaluateRolloverView(b, in, covers)
	}
	p := in.Period
	current, _ := PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	first := current
	found := false
	var ledger NettedLedger
	if in.Ledger != nil {
		ledger = *in.Ledger
	} else {
		ledger = NewNettedLedger(in.Transactions)
	}
	for _, t := range ledger.transactions {
		_, ok, err := ContributionChecked(b, t, func(id string) bool { return covers[id] })
		if err != nil {
			return RolloverView{}, fmt.Errorf("envelope first contribution: %w", err)
		}
		if ok && (!found || t.Date.Before(first)) {
			first, found = t.Date, true
		}
	}
	if found {
		first, _ = PeriodRangeAnchored(b.Period, first, p.WeekStart, p.PayCycleAnchor)
	}
	if !b.RolloverStart.IsZero() {
		start, _ := PeriodRangeAnchored(b.Period, b.RolloverStart, p.WeekStart, p.PayCycleAnchor)
		if start.After(first) {
			first = start
		}
	}
	if first.After(current) {
		first = current
	}
	b.RolloverStart = first
	return EvaluateRolloverView(b, in, covers)
}
