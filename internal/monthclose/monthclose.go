// SPDX-License-Identifier: MIT

// Package monthclose computes the guided month-close picture for a budget
// period: what ended over budget, what money went unused, how actual income
// compared to the expected basis, whether the plan is over-assigned, and the
// legitimate ways to resolve it. It is pure aggregation over already-evaluated
// budget statuses — the UI composes the existing primitives (cover-all,
// rollover, one-time boosts, leftover sweep) around this summary.
package monthclose

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
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
func (s Summary) IncomeDeltaMinor() int64 { return s.ActualIncomeMinor - s.ExpectedIncomeMinor }

// Clean reports whether the period closes with nothing demanding a decision:
// no overspent budgets and no over-assignment.
func (s Summary) Clean() bool { return len(s.Overspends) == 0 && s.OverAssignedMinor == 0 }

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
		switch {
		case st.Remaining.Amount < 0:
			over := -st.Remaining.Amount
			s.Overspends = append(s.Overspends, Item{BudgetID: st.Budget.ID, Name: name(st.Budget), Minor: over})
			s.TotalOverMinor += over
		case st.Remaining.Amount > 0:
			s.Leftovers = append(s.Leftovers, Item{BudgetID: st.Budget.ID, Name: name(st.Budget), Minor: st.Remaining.Amount})
			s.TotalLeftMinor += st.Remaining.Amount
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

// CopyBoosts computes the "copy last month with exceptions" plan: budget limits
// already persist period to period, so what a new period loses is last period's
// ONE-TIME top-ups (PeriodBoosts). For every budget whose last-period boost is
// non-zero and not excluded, the result maps budget ID → that boost amount,
// ready to be written as this period's boost. Budgets already carrying a boost
// this period are skipped — copying must never stack on a manual top-up.
// periodStarts resolves each budget's own last/this period-start dates (budgets
// can run weekly, monthly, or quarterly, so the key differs per budget).
func CopyBoosts(budgets []domain.Budget, periodStarts func(domain.Budget) (last, this time.Time), exclude map[string]bool) map[string]int64 {
	out := map[string]int64{}
	for _, b := range budgets {
		if exclude[b.ID] {
			continue
		}
		lastStart, thisStart := periodStarts(b)
		last := b.PeriodBoost(lastStart)
		if last == 0 || b.PeriodBoost(thisStart) != 0 {
			continue
		}
		out[b.ID] = last
	}
	return out
}
