// SPDX-License-Identifier: MIT

package formula

import "fmt"

// Program is a parsed formula ready for repeated evaluation. Compile once
// (e.g. when a molecule or widget binding is saved or first seen), then call
// Eval per render — skipping tokenize+parse each time. A Program is immutable
// and safe for concurrent use.
type Program struct {
	src string
	ast Node
}

// Compile parses an expression into a reusable Program.
func Compile(input string) (*Program, error) {
	ast, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return &Program{src: input, ast: ast}, nil
}

// Source returns the expression text the Program was compiled from.
func (p *Program) Source() string { return p.src }

// Eval evaluates the compiled formula against env, with the same result
// contract as the package-level Eval (scalar Value, finite numbers only).
func (p *Program) Eval(env Env) (Value, error) {
	return finishEval(p.ast, env)
}

// References returns the distinct variable identifiers the compiled formula
// reads, in sorted order.
func (p *Program) References() []string { return collectRefs(p.ast) }

// Validate checks that expr parses and — when known is non-nil — that every
// variable it references is known. It never evaluates anything, so it is safe
// on partial or absent data: the right check for a save path, where "does this
// formula make sense?" must not depend on today's values.
func Validate(expr string, known func(name string) bool) error {
	ast, err := Parse(expr)
	if err != nil {
		return err
	}
	if known == nil {
		return nil
	}
	for _, ref := range collectRefs(ast) {
		if !known(ref) {
			return fmt.Errorf("formula: unknown variable %q", ref)
		}
	}
	return nil
}

// FunctionDoc describes one built-in function: the single source of truth for
// what the language offers, so editors, docs, and AI tool descriptions can
// enumerate the real function set instead of hand-maintaining drifting copies.
type FunctionDoc struct {
	Name      string
	Signature string
	Doc       string
}

// functionDocs mirrors evalCall's dispatch, one entry per function. A test
// cross-checks that every documented name actually evaluates.
var functionDocs = []FunctionDoc{
	{"sum", "sum(a, b, …)", "Total of the arguments; sum() is 0."},
	{"avg", "avg(a, b, …)", "Mean of the arguments (at least one)."},
	{"min", "min(a, b, …)", "Smallest argument (at least one)."},
	{"max", "max(a, b, …)", "Largest argument (at least one)."},
	{"count", "count(a, b, …)", "How many arguments were given."},
	{"abs", "abs(x)", "Absolute value."},
	{"round", "round(x)", "Nearest integer, halves away from zero."},
	{"floor", "floor(x)", "Round down."},
	{"ceil", "ceil(x)", "Round up."},
	{"clamp", "clamp(x, lo, hi)", "x bounded to [lo, hi]."},
	{"safediv", "safediv(a, b, fallback)", "a / b, or fallback when the division can't produce a finite number."},
	{"if", "if(cond, then, else)", "then when cond is truthy, otherwise else; only the taken branch is evaluated."},
	{"and", "and(a, b, …)", "True when every argument is truthy; stops at the first falsy one."},
	{"or", "or(a, b, …)", "True when any argument is truthy; stops there."},
	{"not", "not(a)", "Logical negation of a truthy value."},
	{"contains", "contains(haystack, needle)", "Case-insensitive substring test on strings."},
	{"lower", "lower(s)", "Lowercase a string."},
}

// Functions lists the built-in functions in a stable order.
func Functions() []FunctionDoc {
	out := make([]FunctionDoc, len(functionDocs))
	copy(out, functionDocs)
	return out
}
