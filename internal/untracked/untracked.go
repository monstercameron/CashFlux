// SPDX-License-Identifier: MIT

// Package untracked computes the "spending no budget watches" picture for the
// /budgets surface: which expense categories have spending that no budget counts,
// what tracking each would cost, and — critically — what tracking them does to a
// zero-based plan before anything is written.
//
// The strip that surfaces this today is a per-category invitation ("Budget
// this"), and it hides two consequences that only appear once you act:
//
//   - Adding a category to an EXISTING budget makes its spending count without
//     touching that budget's limit, so a healthy budget can flip straight to
//     "over" — not because anything financial changed, but because it was told to
//     watch money already being spent. Impact reports that, and RaiseMinor exists
//     so the caller can offer to fix it in the same gesture.
//   - In zero-based every budget created or raised increases the assigned total,
//     which drives To Assign further negative. Making the plan HONEST therefore
//     makes the headline figure look worse. That is correct, and it must be said
//     before the write rather than discovered after it.
//
// It is pure aggregation over transactions, categories and budgets; the UI owns
// the sheet, the pickers and the writes.
package untracked

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Candidate is one expense category with spending that no budget counts.
type Candidate struct {
	CategoryID string
	Name       string
	// SpentMinor is spend in the scanned window, in base minor units.
	SpentMinor int64
	// LastSeen is the most recent transaction date in the window. The sheet shows
	// it because a 12-month scan surfaces yearly obligations whose last hit may be
	// eleven months ago, and "$1,500 untracked" reads very differently once you
	// know whether that was last week or last November.
	LastSeen time.Time
	// SuggestMinor is the amount to seed the row with, and Period the cadence a new
	// budget should take. Both come from the caller's Hint when it has one — a
	// detected recurring commitment knows its exact scheduled amount and rhythm,
	// which beats any average — and fall back to the window's own spend.
	SuggestMinor int64
	Period       domain.Period
	// FromHint records which of those two produced SuggestMinor, so the sheet can
	// say "from your recurring schedule" rather than presenting a guess and a known
	// figure in the same voice.
	FromHint bool
}

// Hint supplies a better amount and cadence for a category than raw window spend
// — in practice the recurring detector's schedule. Returning ok=false falls back.
type Hint func(categoryID string) (minor int64, period domain.Period, ok bool)

// Scan returns the untracked expense categories with spending in [from, to),
// largest first, then by name so the order is stable across renders.
//
// A category is tracked when ANY budget lists it (primary or extra), so the test
// is over TrackedCategoryIDs rather than the primary CategoryID alone — a budget
// that watches three categories covers all three. Income and transfers never
// qualify: IsExpense is the same predicate the budget evaluator uses, so a
// mortgage payment recorded as a transfer to a liability is out of scope here for
// exactly the reason it is out of scope there.
func Scan(txns []domain.Transaction, cats []domain.Category, budgets []domain.Budget,
	from, to time.Time, rates currency.Rates, base string, hint Hint) []Candidate {

	tracked := map[string]bool{}
	for _, b := range budgets {
		for _, cid := range b.TrackedCategoryIDs() {
			tracked[cid] = true
		}
	}

	spent := map[string]int64{}
	last := map[string]time.Time{}
	for _, t := range txns {
		if !t.IsExpense() || t.CategoryID == "" || tracked[t.CategoryID] {
			continue
		}
		if !dateutil.InRange(t.Date, from, to) {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), base)
		if err != nil {
			// One unconvertible transaction must not blank the whole category; skip
			// just this line, the same way the summary's FX handling does.
			continue
		}
		spent[t.CategoryID] += conv.Amount
		if t.Date.After(last[t.CategoryID]) {
			last[t.CategoryID] = t.Date
		}
	}

	name := map[string]string{}
	for _, c := range cats {
		name[c.ID] = c.Name
	}

	out := make([]Candidate, 0, len(spent))
	for id, amt := range spent {
		if amt <= 0 {
			continue
		}
		c := Candidate{
			CategoryID:   id,
			Name:         name[id],
			SpentMinor:   amt,
			LastSeen:     last[id],
			SuggestMinor: amt,
			Period:       domain.PeriodMonthly,
		}
		if hint != nil {
			if minor, p, ok := hint(id); ok && minor > 0 {
				c.SuggestMinor, c.Period, c.FromHint = minor, p, true
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SpentMinor != out[j].SpentMinor {
			return out[i].SpentMinor > out[j].SpentMinor
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Choice is one row's resolved decision: track this category, at this amount,
// into a new budget or an existing one.
type Choice struct {
	CategoryID string
	// AmountMinor is the limit for a new budget, or the amount to raise an
	// existing destination by when Raise is set.
	AmountMinor int64
	// BudgetID empty means "create a new budget"; otherwise the destination.
	BudgetID string
	// Raise asks for the destination budget's limit to grow by AmountMinor.
	// Meaningless for a new budget, which is created AT that amount.
	Raise bool
}

// Effect is what a set of choices does to the plan, computed before any write.
type Effect struct {
	// Categories is how many rows are being tracked.
	Categories int
	// NewBudgets and Raises split the work by kind.
	NewBudgets int
	Raises     int
	// TrackedMinor is the spend being brought into the plan — the honesty gain.
	TrackedMinor int64
	// AssignedDeltaMinor is how much the assigned total grows. A row pointed at an
	// existing budget WITHOUT Raise adds nothing to it: the spending starts
	// counting against a limit that did not move, which is the overspend trap.
	AssignedDeltaMinor int64
	// ToAssignBefore/After are the zero-based figures either side of the change.
	// Both are signed; negative is over-assigned.
	ToAssignBeforeMinor int64
	ToAssignAfterMinor  int64
}

// Impact projects a set of choices onto the plan.
//
// pool is the income basis (plus any rollover) and assigned the current total
// across budgets and savings — the same two numbers the summary's To Assign is
// built from, so this can never disagree with the figure on screen.
func Impact(choices []Choice, pool, assigned int64) Effect {
	e := Effect{
		ToAssignBeforeMinor: budgeting.ToAssign(pool, assigned),
	}
	for _, c := range choices {
		e.Categories++
		e.TrackedMinor += c.AmountMinor
		switch {
		case c.BudgetID == "":
			e.NewBudgets++
			e.AssignedDeltaMinor += c.AmountMinor
		case c.Raise:
			e.Raises++
			e.AssignedDeltaMinor += c.AmountMinor
		}
	}
	e.ToAssignAfterMinor = budgeting.ToAssign(pool, assigned+e.AssignedDeltaMinor)
	return e
}

// OverspendRisk reports the destinations that would be pushed over their limit by
// the categories aimed at them WITHOUT a matching raise — the trap this package
// exists to make visible. spentOf and limitOf report a budget's current figures.
//
// Returned budget IDs are sorted so the warning is stable between renders.
func OverspendRisk(choices []Choice, spentOf, limitOf func(budgetID string) int64) []string {
	add := map[string]int64{}
	for _, c := range choices {
		if c.BudgetID == "" || c.Raise {
			continue
		}
		add[c.BudgetID] += c.AmountMinor
	}
	var out []string
	for id, extra := range add {
		if spentOf(id)+extra > limitOf(id) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// TotalSpent sums what the candidates spent in the scanned window.
//
// It exists so the hero and the unbudgeted strip cannot disagree about how much
// money left the household outside any budget. Both used to derive that
// separately — the strip inline in view code, the hero not at all — and two
// places computing one fact is how they drift.
func TotalSpent(cands []Candidate) int64 {
	var total int64
	for _, c := range cands {
		total += c.SpentMinor
	}
	return total
}
