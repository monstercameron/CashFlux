// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BudgetBaseAt returns the base limit in force at start, before carry and boosts.
// Without recorded history the standing limit is used. Invalid history fails
// visibly rather than silently picking a different base.
func BudgetBaseAt(b domain.Budget, start time.Time) (money.Money, error) {
	amount := b.Limit.Amount
	latest := ""
	key := start.Format(time.DateOnly)
	for date, value := range b.LimitHistory {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return money.Money{}, fmt.Errorf("limit history date %q: %w", date, err)
		}
		if date <= key && date > latest {
			latest = date
			amount = value
		}
	}
	return money.New(amount, b.Limit.Currency), nil
}

// EvaluateRolloverView evaluates the budget's full carry chain once and returns
// bounded display history. For a legacy budget without a recorded first period,
// the chain starts with the period immediately before the viewed one.
func EvaluateRolloverView(b domain.Budget, in ViewInput, covers map[string]bool) (RolloverView, error) {
	p := in.Period
	current, _ := PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	first := b.RolloverStart
	assumed := first.IsZero()
	if first.IsZero() {
		first, _ = PreviousPeriodRangeAnchored(b.Period, current, p.WeekStart, p.PayCycleAnchor)
	}
	first, _ = PeriodRangeAnchored(b.Period, first, p.WeekStart, p.PayCycleAnchor)
	if first.After(current) {
		first = current
	}
	ledger := NewNettedLedger(in.Transactions)
	var inputs []RolloverPeriod
	for start := first; !start.After(current); {
		_, end := PeriodRangeAnchored(b.Period, start, p.WeekStart, p.PayCycleAnchor)
		if !end.After(start) {
			return RolloverView{}, fmt.Errorf("rollover period did not advance")
		}
		base, err := BudgetBaseAt(b, start)
		if err != nil {
			return RolloverView{}, err
		}
		base, err = base.Add(money.New(b.PeriodBoosts[PeriodKeyWithWeekStart(b.Period, start, p.PayCycleAnchor, p.WeekStart)], base.Currency))
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover period boost: %w", err)
		}
		spend, err := SpentFromNetted(b, ledger, start, end, in.Rates, covers)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover spend %s: %w", start.Format(time.DateOnly), err)
		}
		smoothingBudget := b
		smoothingBudget.CategoryIDs = ScopeOf(b, covers).CategoryIDs
		offset, _, err := SmoothingLandingItemsWithRates(smoothingBudget, in.Recurring, in.Transactions, start, end, in.Rates)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover smoothing: %w", err)
		}
		if offset > spend.Amount {
			offset = spend.Amount
		}
		if offset > 0 {
			spend, err = spend.Sub(money.New(offset, spend.Currency))
			if err != nil {
				return RolloverView{}, fmt.Errorf("rollover smoothed spend: %w", err)
			}
		}
		inputs = append(inputs, RolloverPeriod{Start: start, Base: base, Spend: spend})
		start = end
	}
	chain, err := BuildRolloverChain(inputs, b.RolloverCapPeriods)
	if err != nil {
		return RolloverView{}, err
	}
	out := RolloverView{Start: first, StartAssumed: assumed, Periods: len(chain), Current: chain[len(chain)-1], TotalBase: money.Zero(b.Limit.Currency), TotalSpent: money.Zero(b.Limit.Currency)}
	for _, state := range chain {
		out.TotalBase, err = out.TotalBase.Add(state.Base)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover base total: %w", err)
		}
		out.TotalSpent, err = out.TotalSpent.Add(state.Spend)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover spend total: %w", err)
		}
	}
	from := len(chain) - 4
	if from < 0 {
		from = 0
	}
	out.Prior = append([]RolloverPeriodState(nil), chain[from:len(chain)-1]...)
	return out, nil
}
