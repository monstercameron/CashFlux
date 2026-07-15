// SPDX-License-Identifier: MIT

package budgeting

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
