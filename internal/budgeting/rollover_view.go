// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MaxRolloverPeriods bounds work for corrupt or mistyped start dates (about 46
// years of weekly periods). Longer histories require repairing the start date.
const MaxRolloverPeriods = 2400

// ErrRolloverHistoryTooLong marks an incomplete carry calculation, never a
// silently truncated balance.
var ErrRolloverHistoryTooLong = errors.New("rollover history exceeds 2400 periods; check the start date")

// BudgetBaseAt returns the base limit in force at start, before carry and boosts.
// Before the first recorded change, the oldest recorded base is used; with no
// history the standing limit is used. Invalid history fails
// visibly rather than silently picking a different base.
func BudgetBaseAt(b domain.Budget, start time.Time) (money.Money, error) {
	amount := b.Limit.Amount
	latest := ""
	earliest := ""
	initial := amount
	key := start.Format(time.DateOnly)
	for date, value := range b.LimitHistory {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return money.Money{}, fmt.Errorf("limit history date %q: %w", date, err)
		}
		if earliest == "" || date < earliest {
			earliest = date
			initial = value
		}
		if date <= key && date > latest {
			latest = date
			amount = value
		}
	}
	if latest == "" && earliest != "" {
		amount = initial
	}
	return money.New(amount, b.Limit.Currency), nil
}

// EvaluateRolloverView evaluates the budget's full carry chain once and returns
// bounded display history. For a legacy budget without a recorded first period,
// the chain starts with the period immediately before the viewed one.
func EvaluateRolloverView(b domain.Budget, in ViewInput, covers map[string]bool) (RolloverView, error) {
	b = b.WithBaseCurrency(in.Base)
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
	ledger := NewNettedLedgerWithLinks(in.Transactions, nil)
	if in.Ledger != nil {
		ledger = *in.Ledger
	} else {
		ledger = NewNettedLedger(in.Transactions)
	}
	// Bucket the immutable netted ledger once, instead of scanning it per period.
	spends := make(map[time.Time]money.Money)
	postedByPeriod := make(map[time.Time][]domain.Transaction)
	for _, t := range ledger.transactions {
		if t.Date.Before(first) {
			continue
		}
		ps, _ := PeriodRangeAnchored(b.Period, t.Date, p.WeekStart, p.PayCycleAnchor)
		if ps.After(current) {
			continue
		}
		amount, ok, err := ContributionChecked(b, t, func(id string) bool { return covers[id] })
		if err != nil {
			return RolloverView{}, err
		}
		if !ok {
			continue
		}
		postedByPeriod[ps] = append(postedByPeriod[ps], t)
		converted, err := in.Rates.Convert(amount, b.Limit.Currency)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover contribution %q: %w", t.ID, err)
		}
		total := spends[ps]
		if total.Currency == "" {
			total = money.Zero(b.Limit.Currency)
		}
		spends[ps], err = total.Add(converted)
		if err != nil {
			return RolloverView{}, fmt.Errorf("rollover spend bucket: %w", err)
		}
	}
	var inputs []RolloverPeriod
	for start := first; !start.After(current); {
		if len(inputs) >= MaxRolloverPeriods {
			return RolloverView{}, ErrRolloverHistoryTooLong
		}
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
		spend := spends[start]
		if spend.Currency == "" {
			spend = money.Zero(b.Limit.Currency)
		}
		smoothingBudget := b
		smoothingBudget.CategoryIDs = ScopeOf(b, covers).CategoryIDs
		offset, _, err := SmoothingLandingItemsWithRates(smoothingBudget, in.Recurring, postedByPeriod[start], start, end, in.Rates)
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
