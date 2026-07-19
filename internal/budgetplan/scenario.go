// SPDX-License-Identifier: MIT

package budgetplan

// BudgetPlan is one budget's planned outflow for a single month, in
// base-currency minor units, listed in the order funding is applied — the first
// entry is funded first. The caller decides the order (the grid supplies its
// display order), so the priority is explicit rather than hidden.
type BudgetPlan struct {
	BudgetID  string
	Name      string
	PlanMinor int64
}

// ScenarioInput is a single month's what-if: a baseline income figure with a
// hypothetical delta, the month's plans in funding-priority order, and an
// optional bump to one budget's own plan (e.g. "rent +$200"). All amounts are
// base-currency minor units.
type ScenarioInput struct {
	IncomeMinor        int64        // baseline income available this month
	IncomeDeltaMinor   int64        // hypothetical change to income (+/-)
	Plans              []BudgetPlan // funded first-to-last in this order
	TargetBudgetID     string       // budget whose plan the category delta bumps ("" = none)
	CategoryDeltaMinor int64        // hypothetical change to the target budget's plan
}

// Funded is one budget's outcome under the scenario. PlanMinor is its plan after
// any category delta; FundedMinor is how much the adjusted income covered;
// ShortfallMinor (= PlanMinor − FundedMinor) is positive exactly when the budget
// is underfunded.
type Funded struct {
	BudgetID       string
	Name           string
	PlanMinor      int64
	FundedMinor    int64
	ShortfallMinor int64
}

// Underfunded reports whether this budget could not be fully covered.
func (f Funded) Underfunded() bool { return f.ShortfallMinor > 0 }

// ScenarioResult is the month's outcome: the adjusted income, the total plan,
// the per-budget funding waterfall, and the aggregate shortfall.
type ScenarioResult struct {
	AdjustedIncomeMinor int64
	TotalPlanMinor      int64
	ShortfallMinor      int64
	Funded              []Funded
	Underfunded         []string // budgetIDs with a shortfall, in input order
}

// IsUnderfunded reports whether the given budget went underfunded in this result.
func (r ScenarioResult) IsUnderfunded(budgetID string) bool {
	for _, id := range r.Underfunded {
		if id == budgetID {
			return true
		}
	}
	return false
}

// Evaluate runs the funding waterfall for one month: the adjusted income
// (income + delta, floored at zero) is poured into the plans in order, each
// fully funded until the income runs out; the budget where it runs dry is
// partially funded and every plan after it is unfunded. A budget is
// "underfunded" when it cannot be fully covered. Deterministic and
// order-explicit — nothing is hidden, so a caller can show exactly which cells
// the scenario cuts and by how much.
func Evaluate(in ScenarioInput) ScenarioResult {
	income := in.IncomeMinor + in.IncomeDeltaMinor
	if income < 0 {
		income = 0
	}
	res := ScenarioResult{AdjustedIncomeMinor: income}
	remaining := income
	for _, p := range in.Plans {
		plan := p.PlanMinor
		if p.BudgetID == in.TargetBudgetID && in.TargetBudgetID != "" {
			plan += in.CategoryDeltaMinor
		}
		if plan < 0 {
			plan = 0
		}
		res.TotalPlanMinor += plan

		funded := plan
		if funded > remaining {
			funded = remaining
		}
		if funded < 0 {
			funded = 0
		}
		remaining -= funded
		short := plan - funded

		res.Funded = append(res.Funded, Funded{
			BudgetID: p.BudgetID, Name: p.Name,
			PlanMinor: plan, FundedMinor: funded, ShortfallMinor: short,
		})
		if short > 0 {
			res.ShortfallMinor += short
			res.Underfunded = append(res.Underfunded, p.BudgetID)
		}
	}
	return res
}
