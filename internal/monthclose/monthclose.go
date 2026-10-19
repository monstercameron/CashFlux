// SPDX-License-Identifier: MIT

// Package monthclose computes the guided month-close picture for a budget
// period: what ended over budget, what money went unused, how actual income
// compared to the expected basis, whether the plan is over-assigned, and the
// legitimate ways to resolve it. It is pure aggregation over already-evaluated
// budget statuses — the UI composes the existing primitives (cover-all,
// rollover, one-time boosts, leftover sweep) around this summary.
package monthclose

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Item is one budget's contribution to a month-close step: which budget, its
// display name, and the amount at stake (always >= 0, base minor units).
type Item struct {
	BudgetID string
	Name     string
	Minor    int64
}

// Summary is the computed month-close picture for one budget period.
type Summary struct {
	// Error identifies an arithmetic failure; totals must not be displayed then.
	Error error
	// Overspends lists budgets that ended over their cap, largest overage first.
	Overspends []Item
	// Leftovers lists budgets that ended with money unspent, largest first.
	Leftovers []Item
	// TotalOverMinor and TotalLeftMinor are the two lists' sums.
	TotalOverMinor int64
	TotalLeftMinor int64
	// ExpectedIncomeMinor is the income basis the plan was built on;
	// ActualIncomeMinor is what income transactions actually recorded this period.
	ExpectedIncomeMinor int64
	ActualIncomeMinor   int64
	// OverAssignedMinor is how far the plan exceeds the income basis (0 = fits).
	OverAssignedMinor int64
	// RolloverOn reports whether leftover rollover into next period is enabled.
	RolloverOn bool
}

// IncomeDeltaMinor is actual minus expected income: positive means the period
// brought in more than the plan assumed.
func (s Summary) IncomeDeltaMinor() int64 {
	delta, _ := s.IncomeDeltaChecked()
	return delta
}

// IncomeDeltaChecked returns the income difference without wrapping money.
func (s Summary) IncomeDeltaChecked() (int64, error) {
	return money.Sub(s.ActualIncomeMinor, s.ExpectedIncomeMinor)
}

// Clean reports whether the period closes with nothing demanding a decision:
// no overspent budgets and no over-assignment.
func (s Summary) Clean() bool {
	return s.Error == nil && len(s.Overspends) == 0 && s.OverAssignedMinor == 0
}

// Build assembles the month-close summary from evaluated statuses. nameOf
// resolves a budget's display name (nil falls back to the budget's Name field).
// Both item lists sort by amount descending, then by name for a stable order.
func Build(statuses []budgeting.Status, nameOf func(domain.Budget) string, expectedIncome, actualIncome, overAssigned int64, rolloverOn bool) Summary {
	if overAssigned < 0 {
		overAssigned = 0
	}
	s := Summary{
		ExpectedIncomeMinor: expectedIncome,
		ActualIncomeMinor:   actualIncome,
		OverAssignedMinor:   overAssigned,
		RolloverOn:          rolloverOn,
	}
	name := func(b domain.Budget) string {
		if nameOf != nil {
			if n := nameOf(b); n != "" {
				return n
			}
		}
		return b.Name
	}
	for _, st := range statuses {
		if st.Budget.IsSaving() {
			continue
		}
		switch {
		case st.Remaining.Amount < 0:
			over, err := money.Neg(st.Remaining.Amount)
			if err != nil {
				s.Error = errors.Join(s.Error, err)
				continue
			}
			s.Overspends = append(s.Overspends, Item{BudgetID: st.Budget.ID, Name: name(st.Budget), Minor: over})
			total, err := money.Add(s.TotalOverMinor, over)
			if err != nil {
				s.Error = errors.Join(s.Error, err)
			} else {
				s.TotalOverMinor = total
			}
		case st.Remaining.Amount > 0:
			s.Leftovers = append(s.Leftovers, Item{BudgetID: st.Budget.ID, Name: name(st.Budget), Minor: st.Remaining.Amount})
			total, err := money.Add(s.TotalLeftMinor, st.Remaining.Amount)
			if err != nil {
				s.Error = errors.Join(s.Error, err)
			} else {
				s.TotalLeftMinor = total
			}
		}
	}
	byAmount := func(items []Item) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Minor != items[j].Minor {
				return items[i].Minor > items[j].Minor
			}
			return items[i].Name < items[j].Name
		})
	}
	byAmount(s.Overspends)
	byAmount(s.Leftovers)
	return s
}

// Over-assignment resolution kinds, in the order they should be offered.
const (
	// ResolveReduce trims another category's limit to make the plan fit.
	ResolveReduce = "reduce"
	// ResolveIncome revisits the expected-income basis (raise or correct it).
	ResolveIncome = "income"
	// ResolveRollover turns on leftover rollover so unspent money absorbs the gap.
	ResolveRollover = "rollover"
	// ResolveDefer records the choice to leave it unresolved for now.
	ResolveDefer = "defer"
)

// Resolutions returns the over-assignment choices that actually apply to this
// summary, in presentation order. Empty when the plan isn't over-assigned.
// Reduce is only offered when at least one budget has money to reclaim, and
// rollover only when it is off and there is leftover for it to carry.
func Resolutions(s Summary) []string {
	if s.OverAssignedMinor <= 0 {
		return nil
	}
	var out []string
	if len(s.Leftovers) > 0 {
		out = append(out, ResolveReduce)
	}
	out = append(out, ResolveIncome)
	if !s.RolloverOn && s.TotalLeftMinor > 0 {
		out = append(out, ResolveRollover)
	}
	return append(out, ResolveDefer)
}

// CarryTargets resolves the (source, target) period starts CopyBoosts should use
// when carrying one-time top-ups forward from the period being reviewed.
//
// The direction matters and used to be wrong. The caller derived (last, this) —
// carry the PREVIOUS period's top-ups into the CURRENT one — which is coherent
// only if the flow is opened at the start of a period. It is not: the offer
// appears in a period's final days, or on one that has already closed. Reviewing
// August on the 31st therefore wrote July's top-ups into August, a period with
// nothing left to spend them in; reviewing a closed August from September did
// the same, because the anchor falls back to the window's start either way. The
// step could not land usefully in the window it is offered (Cam, 2026-08-31).
//
// Carrying forward is the only direction that makes sense at a period's end, so
// source is the reviewed period and target is the one after it. `end` from
// PeriodRange is exclusive, so it is already a reference date inside the next
// period — no calendar arithmetic here, which is what keeps weekly, biweekly,
// monthly and quarterly budgets all correct through the same call.
func CarryTargets(p domain.Period, ref time.Time, weekStart time.Weekday) (source, target time.Time) {
	return CarryTargetsAnchored(p, ref, weekStart, time.Time{})
}

// CarryTargetsAnchored resolves the reviewed and following period on a grid
// that includes the configured biweekly pay-cycle anchor.
func CarryTargetsAnchored(p domain.Period, ref time.Time, weekStart time.Weekday, payAnchor time.Time) (source, target time.Time) {
	thisStart, thisEnd := budgeting.PeriodRangeAnchored(p, ref, weekStart, payAnchor)
	nextStart, _ := budgeting.PeriodRangeAnchored(p, thisEnd, weekStart, payAnchor)
	return thisStart, nextStart
}

// CopyBoosts copies only explicitly recorded unfunded PeriodTopups. Covers,
// funded raises and opaque legacy adjustments remain in their original periods.
// Excluding any row therefore cannot replay just one leg of a money move.
// Existing target adjustments or top-ups prevent stacking. periodStarts resolves
// each budget's own source and target keys, including weekly and biweekly grids.
func CopyBoosts(budgets []domain.Budget, periodStarts func(domain.Budget) (last, this time.Time), exclude map[string]bool) map[string]int64 {
	out := map[string]int64{}
	for _, b := range budgets {
		if exclude[b.ID] {
			continue
		}
		lastStart, thisStart := periodStarts(b)
		// Only explicitly identified unfunded top-ups may be copied. Legacy
		// aggregates can contain either leg of a cover and must stay put.
		last := b.PeriodTopups[lastStart.Format(time.DateOnly)]
		if last <= 0 || b.PeriodBoost(thisStart) != 0 || b.PeriodTopups[thisStart.Format(time.DateOnly)] != 0 {
			continue
		}
		out[b.ID] = last
	}
	return out
}

// ApplyCopyBoosts returns the budget updates for the copy plan, keyed to each
// budget's returned target period. Existing target boosts are never stacked.
func ApplyCopyBoosts(budgets []domain.Budget, periodStarts func(domain.Budget) (source, target time.Time), exclude map[string]bool) ([]domain.Budget, error) {
	plan := CopyBoosts(budgets, periodStarts, exclude)
	writes := make([]domain.Budget, 0, len(plan))
	for _, budget := range budgets {
		amount, ok := plan[budget.ID]
		if !ok {
			continue
		}
		_, target := periodStarts(budget)
		updated, err := budget.WithPeriodBoostChecked(target, amount)
		if err != nil {
			return nil, fmt.Errorf("copy top-up for budget %q: %w", budget.ID, err)
		}
		writes = append(writes, updated)
	}
	return writes, nil
}
