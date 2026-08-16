// SPDX-License-Identifier: MIT

package budgeting

import "github.com/monstercameron/CashFlux/internal/domain"

// Methodology is the budgeting approach a household uses. It shapes the Budgets
// view's affordances; the per-category limit evaluation (Evaluate) is the same
// across methodologies.
type Methodology string

const (
	// MethodSimple uses independent per-category limits (the default behavior).
	MethodSimple Methodology = "simple"
	// MethodZeroBased assigns every dollar of income to a budget, so the view
	// surfaces how much income is still unassigned ("to assign").
	MethodZeroBased Methodology = "zero-based"
	// MethodEnvelope treats budgets as envelopes that carry their unspent balance
	// forward. Reserved for a future view; selecting it behaves like simple today.
	MethodEnvelope Methodology = "envelope"
	// MethodFlex (BG2) manages one pooled "flex" number for all day-to-day
	// discretionary spending, while fixed commitments render as expected-vs-actual
	// checkoffs and non-monthly costs show their smoothed accrual. The view is
	// driven by each category's domain.CategoryClass rather than per-budget limits.
	MethodFlex Methodology = "flex"
)

// Valid reports whether m is a known methodology.
func (m Methodology) Valid() bool {
	switch m {
	case MethodSimple, MethodZeroBased, MethodEnvelope, MethodFlex:
		return true
	default:
		return false
	}
}

// ParseMethodology returns the methodology for s, defaulting to MethodSimple for
// an empty or unknown value — so older datasets without the field, and any
// future value this build doesn't know, behave as the safe default.
func ParseMethodology(s string) Methodology {
	if m := Methodology(s); m.Valid() {
		return m
	}
	return MethodSimple
}

// ToAssign returns income minus the total budgeted — the amount of income still
// unassigned under zero-based budgeting. A negative result means over-assigned
// (the budgets exceed income). Amounts are minor units in the base currency.
func ToAssign(income, totalBudgeted int64) int64 {
	return income - totalBudgeted
}

// MethodImpact describes what changing the HOUSEHOLD's budgeting method will
// actually touch (C590).
//
// The page offers two method controls — a global one in Budget settings and a
// per-budget one in Add/Edit — with no statement of the relationship between
// them, so a user can change the whole page believing they are editing one card.
// The difference is real: a budget that sets its own method keeps it, and a
// global change moves only the budgets that were following along.
type MethodImpact struct {
	// Following counts budgets with no per-budget override. These are the ones a
	// global change moves.
	Following int
	// Overriding counts budgets that set their own method and will not change.
	Overriding int
	// Overrides tallies those budgets by the method they chose, so the UI can name
	// what stays put instead of only counting it.
	Overrides map[Methodology]int
}

// Total is how many budgets were considered.
func (i MethodImpact) Total() int { return i.Following + i.Overriding }

// MethodChangeImpact splits budgets into those that follow the household method
// and those that override it. An unknown or empty per-budget value counts as
// following, matching ParseMethodology's rule that anything unrecognised behaves
// as the default rather than as a private setting.
func MethodChangeImpact(budgets []domain.Budget) MethodImpact {
	out := MethodImpact{Overrides: map[Methodology]int{}}
	for _, b := range budgets {
		m := Methodology(b.Methodology)
		if m == "" || !m.Valid() {
			out.Following++
			continue
		}
		out.Overriding++
		out.Overrides[m]++
	}
	return out
}
