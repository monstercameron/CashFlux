// SPDX-License-Identifier: MIT

// Package amountmath evaluates arithmetic typed directly into a money amount
// field (TX16). A user can enter "45.99*3" or "(12+8)*2" and, on blur/Enter,
// have it replaced by the computed result. Evaluation goes through the app's
// own sandboxed formula engine with an EMPTY environment — arithmetic only, no
// variables, no host references — so it inherits the engine's finite-result
// guarantee and can never reach app state.
//
// The package is pure (no syscall/js) and unit-tested on native Go.
package amountmath

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// EvalAmount interprets s as an amount expression and returns the numeric
// result when s is a formula worth evaluating.
//
// Behaviour, matching the amount-field contract:
//
//   - A plain number ("12", "1,234.56", " 45.99 ") is NOT a formula: ok is
//     false and the caller leaves the field's text untouched. Commas are
//     ignored only when deciding plain-ness; nothing is rewritten.
//   - A string containing an arithmetic operator (+, -, *, /, parentheses) is
//     evaluated via formula.Eval with an empty Env. On success the numeric
//     result is returned with ok=true.
//   - A negative result returns ok=false: an amount field holds a magnitude, so
//     "10-20" is left untouched rather than silently producing -10.
//   - Any parse or evaluation failure (junk, unknown names, division by zero,
//     non-finite results) returns ok=false with NO error — the field is simply
//     left as the user typed it (no nag).
//
// Commas are stripped before evaluation so "1,234*2" works like a thousands
// grouped figure; the formula tokenizer has no comma-as-separator ambiguity
// here because EvalAmount never passes multiple arguments.
func EvalAmount(s string) (float64, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, false
	}

	// Strip thousands-group commas so "1,234.56" and "1,234*2" both work.
	expr := strings.ReplaceAll(trimmed, ",", "")

	// A plain number carries no operator: pass it through untouched (ok=false)
	// so the caller does not rewrite what the user already typed correctly.
	if !containsOperator(expr) {
		return 0, false
	}

	v, err := formula.Eval(expr, formula.Env{})
	if err != nil {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	// Amount fields hold magnitudes; a negative computed result is not a valid
	// amount, so leave the input untouched rather than coerce it.
	if f < 0 {
		return 0, false
	}
	return f, true
}

// containsOperator reports whether expr has an arithmetic operator that makes it
// a formula rather than a plain number. A leading unary minus alone ("-5") is
// not treated as a formula — that is just a negative number, which the amount
// field rejects anyway. We look for an operator at any interior position.
func containsOperator(expr string) bool {
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '+', '*', '/', '(', ')', '%':
			return true
		case '-':
			// Interior minus (a subtraction) counts; a single leading sign does not.
			if i > 0 {
				return true
			}
		}
	}
	return false
}
