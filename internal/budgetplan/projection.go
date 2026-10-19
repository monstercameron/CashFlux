// SPDX-License-Identifier: MIT

package budgetplan

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
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
	// Error reports arithmetic that could not be represented. Partial amounts are
	// retained for inspection but callers must not present them as complete.
	Error error
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
	projectedRecurring := make(map[string]bool, len(recs))

	for _, r := range recs {
		if !r.Active() || !r.Amount.IsNegative() { // only active bills / money-out are projected spend
			continue
		}
		absolute, err := r.Amount.AbsChecked()
		if err != nil {
			p.Error = fmt.Errorf("recurring %q amount: %w", r.ID, err)
			continue
		}
		minor, err := currency.ConvertBetween(absolute.Amount, r.Amount.Currency, base, rates)
		if err != nil || minor == 0 {
			continue
		}
		if r.NextDue.IsZero() {
			continue
		}
		amt := p.Recurring[r.CategoryID]
		d := r.NextDue.UTC()
		projected := false
		for i := 0; i < projectionSteps && d.Before(yearEnd); i++ {
			if !d.Before(yearStart) {
				if mi := int(d.Month()) - 1; mi >= fromMonth {
					projected = true
					v, err := checkedAdd(amt[mi], minor, base)
					if err != nil {
						p.Error = fmt.Errorf("recurring category %q: %w", r.CategoryID, err)
					} else {
						amt[mi] = v
					}
				}
			}
			next := r.Cadence.Next(d)
			if !next.After(d) { // guard: a cadence that fails to advance would loop
				break
			}
			d = next
		}
		p.Recurring[r.CategoryID] = amt
		if projected {
			projectedRecurring[r.ID] = true
		}
	}

	for _, g := range gs {
		if g.Archived {
			continue
		}
		if recurringID := smoothing.SmoothingRecurringID(g); recurringID != "" && projectedRecurring[recurringID] {
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
				v, err := checkedAdd(byCat[m], minor, base)
				if err != nil {
					p.Error = fmt.Errorf("goal category %q: %w", g.CategoryID, err)
				} else {
					byCat[m] = v
				}
			} else {
				v, err := checkedAdd(byBudget[m], minor, base)
				if err != nil {
					p.Error = fmt.Errorf("goal budget amount: %w", err)
				} else {
					byBudget[m] = v
				}
			}
		}
		if !any {
			continue
		}
		if hasCat {
			cur := p.GoalsByCategory[g.CategoryID]
			if err := mergeMonths(&cur, byCat, base); err != nil {
				p.Error = fmt.Errorf("goal category %q totals: %w", g.CategoryID, err)
			}
			p.GoalsByCategory[g.CategoryID] = cur
		} else {
			for bID, amount := range splitGoalMonths(byBudget, uniqueIDs(g.BudgetIDs)) {
				cur := p.GoalsByBudget[bID]
				if err := mergeMonths(&cur, amount, base); err != nil {
					p.Error = fmt.Errorf("goal budget %q totals: %w", bID, err)
				}
				p.GoalsByBudget[bID] = cur
			}
		}
	}
	return p
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// splitGoalMonths divides each monthly assignment evenly across linked budgets.
// Integer remainders go to the earliest budget ID, so the shares are stable and
// always add back to the exact goal assignment.
func splitGoalMonths(amounts MonthAmounts, budgetIDs []string) map[string]MonthAmounts {
	out := make(map[string]MonthAmounts, len(budgetIDs))
	if len(budgetIDs) == 0 {
		return out
	}
	count := int64(len(budgetIDs))
	for _, id := range budgetIDs {
		out[id] = MonthAmounts{}
	}
	for month, amount := range amounts {
		share, remainder := amount/count, amount%count
		for i, id := range budgetIDs {
			months := out[id]
			months[month] = share
			if int64(i) < remainder {
				months[month]++
			}
			out[id] = months
		}
	}
	return out
}

func checkedAdd(a, b int64, cur string) (int64, error) {
	_ = cur // retained for contextual call-site symmetry; amounts are already base minor units.
	return money.Add(a, b)
}

// ScaleMonthlyIncome returns a monthly income estimate for a half-open date
// window. Partial calendar months are prorated by their actual day count.
func ScaleMonthlyIncome(monthly money.Money, start, end time.Time) (money.Money, error) {
	if !end.After(start) {
		return money.Zero(monthly.Currency), nil
	}
	total := money.Zero(monthly.Currency)
	for cursor := start.UTC(); cursor.Before(end.UTC()); {
		next := time.Date(cursor.Year(), cursor.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		segmentEnd := next
		if end.UTC().Before(segmentEnd) {
			segmentEnd = end.UTC()
		}
		monthStart := time.Date(cursor.Year(), cursor.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthDays := int64(next.Sub(monthStart).Hours() / 24)
		days := int64(segmentEnd.Sub(cursor).Hours() / 24)
		part, err := monthly.MulDiv(days, monthDays, money.RoundHalfAwayFromZero)
		if err != nil {
			return money.Money{}, fmt.Errorf("scale monthly income: %w", err)
		}
		total, err = total.Add(part)
		if err != nil {
			return money.Money{}, fmt.Errorf("sum window income: %w", err)
		}
		cursor = segmentEnd
	}
	return total, nil
}

// PerBudget folds the projection onto grid rows: for each budget it sums the
// recurring outflow and goal contributions of every category the budget covers
// (covers[budgetID] is the budget's rollup category set, exactly as
// budgeting.BuildAnnualGrid consumes it) plus any goals associated with the
// budget directly. The returned per-budget MonthAmounts are the projected spend
// a caller pre-fills into that budget's future cells.
func (p Projection) PerBudget(budgetIDs []string, covers map[string]map[string]bool) (map[string]MonthAmounts, error) {
	out := make(map[string]MonthAmounts, len(budgetIDs))
	for _, bID := range budgetIDs {
		var row MonthAmounts
		for cat := range covers[bID] {
			if ra, ok := p.Recurring[cat]; ok {
				if err := mergeMonths(&row, ra, "USD"); err != nil {
					return nil, err
				}
			}
			if ga, ok := p.GoalsByCategory[cat]; ok {
				if err := mergeMonths(&row, ga, "USD"); err != nil {
					return nil, err
				}
			}
		}
		if gb, ok := p.GoalsByBudget[bID]; ok {
			if err := mergeMonths(&row, gb, "USD"); err != nil {
				return nil, err
			}
		}
		out[bID] = row
	}
	return out, p.Error
}

// TotalUnique sums category projections once across overlapping budget covers,
// plus budget-linked goals once per budget. It is the correct footer total when
// a category is tracked by more than one budget.
func (p Projection) TotalUnique(budgetIDs []string, covers map[string]map[string]bool) (MonthAmounts, error) {
	var total MonthAmounts
	seen := map[string]bool{}
	for _, id := range budgetIDs {
		for categoryID := range covers[id] {
			if seen[categoryID] {
				continue
			}
			seen[categoryID] = true
			if err := mergeMonths(&total, p.Recurring[categoryID], "USD"); err != nil {
				return MonthAmounts{}, err
			}
			if err := mergeMonths(&total, p.GoalsByCategory[categoryID], "USD"); err != nil {
				return MonthAmounts{}, err
			}
		}
		if err := mergeMonths(&total, p.GoalsByBudget[id], "USD"); err != nil {
			return MonthAmounts{}, err
		}
	}
	return total, p.Error
}

func mergeMonths(dst *MonthAmounts, src MonthAmounts, base string) error {
	for month, amount := range src {
		value, err := checkedAdd(dst[month], amount, base)
		if err != nil {
			return err
		}
		dst[month] = value
	}
	return nil
}
