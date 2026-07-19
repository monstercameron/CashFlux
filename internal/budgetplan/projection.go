// SPDX-License-Identifier: MIT

package budgetplan

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
)

// Projection is the future-month projection for one year of the annual grid.
// Recurring is per-category projected OUTFLOW (base-currency minor units) from
// bill schedules; GoalsByCategory and GoalsByBudget are projected goal
// contributions keyed by the goal's spending category or, when it has none, by
// the budgets it is associated with. Only months at or after FromMonth carry
// amounts — earlier months are actuals and are left zero, so a caller can safely
// overlay the projection onto real spend without clobbering the past.
type Projection struct {
	Year            int
	FromMonth       int                     // first projected month index (0-11)
	Recurring       map[string]MonthAmounts // categoryID ("" = uncategorized)
	GoalsByCategory map[string]MonthAmounts // categoryID
	GoalsByBudget   map[string]MonthAmounts // budgetID
}

// projectionSteps bounds the recurring-occurrence walk so a degenerate imported
// schedule (or a very stale NextDue that must fast-forward into the year) can
// never loop forever. Daily cadence across a couple of years of backlog stays
// well inside this.
const projectionSteps = 2000

// Project expands recurring bills and goal contributions into the months of
// `year` at or after fromMonth (0-based). Amounts are converted to base-currency
// minor units through rates. Recurrings with a missing FX rate, and goals whose
// amount can't be converted, are skipped rather than aborting the projection.
//
// A recurring OUTFLOW (negative amount) contributes its absolute amount to every
// occurrence of its cadence that lands in a projected month, keyed by its
// CategoryID. A goal's monthly assignment (goals.MonthlyAssignment, re-derived
// as of each projected month so an amortized "needed / mo" stays honest) is
// added to every projected month, keyed by its CategoryID when set, otherwise by
// each of its BudgetIDs.
func Project(recs []domain.Recurring, gs []domain.Goal, year, fromMonth int, base string, rates currency.Rates) Projection {
	if fromMonth < 0 {
		fromMonth = 0
	}
	p := Projection{
		Year:            year,
		FromMonth:       fromMonth,
		Recurring:       map[string]MonthAmounts{},
		GoalsByCategory: map[string]MonthAmounts{},
		GoalsByBudget:   map[string]MonthAmounts{},
	}
	if fromMonth > 11 {
		return p // whole year is in the past — nothing to project
	}
	yearStart := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC) // exclusive

	for _, r := range recs {
		if !r.Amount.IsNegative() { // only bills / money-out are projected spend
			continue
		}
		minor, err := currency.ConvertBetween(r.Amount.Abs().Amount, r.Amount.Currency, base, rates)
		if err != nil || minor == 0 {
			continue
		}
		if r.NextDue.IsZero() {
			continue
		}
		amt := p.Recurring[r.CategoryID]
		d := r.NextDue.UTC()
		for i := 0; i < projectionSteps && d.Before(yearEnd); i++ {
			if !d.Before(yearStart) {
				if mi := int(d.Month()) - 1; mi >= fromMonth {
					amt[mi] += minor
				}
			}
			next := r.Cadence.Next(d)
			if !next.After(d) { // guard: a cadence that fails to advance would loop
				break
			}
			d = next
		}
		p.Recurring[r.CategoryID] = amt
	}

	for _, g := range gs {
		if g.Archived {
			continue
		}
		var byCat, byBudget MonthAmounts
		hasCat := g.CategoryID != ""
		var any bool
		for m := fromMonth; m < 12; m++ {
			from := time.Date(year, time.Month(m+1), 1, 0, 0, 0, 0, time.UTC)
			assign, ok, err := goals.MonthlyAssignment(g, from)
			if err != nil || !ok || assign.Amount <= 0 {
				continue
			}
			minor, err := currency.ConvertBetween(assign.Amount, assign.Currency, base, rates)
			if err != nil || minor <= 0 {
				continue
			}
			any = true
			if hasCat {
				byCat[m] += minor
			} else {
				byBudget[m] += minor
			}
		}
		if !any {
			continue
		}
		if hasCat {
			cur := p.GoalsByCategory[g.CategoryID]
			addInto(&cur, byCat)
			p.GoalsByCategory[g.CategoryID] = cur
		} else {
			for _, bID := range g.BudgetIDs {
				cur := p.GoalsByBudget[bID]
				addInto(&cur, byBudget)
				p.GoalsByBudget[bID] = cur
			}
		}
	}
	return p
}

// PerBudget folds the projection onto grid rows: for each budget it sums the
// recurring outflow and goal contributions of every category the budget covers
// (covers[budgetID] is the budget's rollup category set, exactly as
// budgeting.BuildAnnualGrid consumes it) plus any goals associated with the
// budget directly. The returned per-budget MonthAmounts are the projected spend
// a caller pre-fills into that budget's future cells.
func (p Projection) PerBudget(budgetIDs []string, covers map[string]map[string]bool) map[string]MonthAmounts {
	out := make(map[string]MonthAmounts, len(budgetIDs))
	for _, bID := range budgetIDs {
		var row MonthAmounts
		for cat := range covers[bID] {
			if ra, ok := p.Recurring[cat]; ok {
				addInto(&row, ra)
			}
			if ga, ok := p.GoalsByCategory[cat]; ok {
				addInto(&row, ga)
			}
		}
		if gb, ok := p.GoalsByBudget[bID]; ok {
			addInto(&row, gb)
		}
		out[bID] = row
	}
	return out
}
