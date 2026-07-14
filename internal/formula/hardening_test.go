// SPDX-License-Identifier: MIT

package formula

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
)

// TestShortCircuit covers the lazy if/and/or semantics: the untaken branch (or
// the arguments after a decisive one) must not evaluate, so guard idioms work.
func TestShortCircuit(t *testing.T) {
	env := Env{Vars: map[string]float64{"count": 0, "total": 100}}
	cases := []struct {
		expr string
		want Value
	}{
		{"if(count == 0, 0, total / count)", float64(0)},   // guarded division
		{"if(count != 0, total / count, -1)", float64(-1)}, // guard inverted
		{"and(count > 0, total / count > 1)", false},       // conjunction guard
		{"or(count == 0, total / count > 1)", true},        // disjunction guard
		{`if(1, "yes", unknown_name)`, "yes"},              // untaken unknown var
	}
	for _, c := range cases {
		v, err := Eval(c.expr, env)
		if err != nil {
			t.Errorf("Eval(%q): %v", c.expr, err)
			continue
		}
		if v != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, v, c.want)
		}
	}
	// The taken path still surfaces real errors.
	if _, err := Eval("if(1, total / count, 0)", env); err == nil {
		t.Error("taken division by zero should still error")
	}
	// Arity checks still fire before anything evaluates.
	for _, expr := range []string{"if(1, 2)", "and(1)", "or(1)"} {
		if _, err := Eval(expr, env); err == nil {
			t.Errorf("Eval(%q) should error on arity", expr)
		}
	}
}

// TestFiniteResultGuard confirms a formula can never return NaN or Inf: the
// top-level result is checked, while clamp/safediv may rescue intermediates.
func TestFiniteResultGuard(t *testing.T) {
	env := Env{Vars: map[string]float64{"inf": math.Inf(1), "nan": math.NaN(), "big": 1e308}}
	for _, expr := range []string{"big * 10", "inf - inf", "nan", "inf", "-inf"} {
		if v, err := Eval(expr, env); err == nil {
			t.Errorf("Eval(%q) = %v, want non-finite error", expr, v)
		}
	}
	// Rescued intermediates are fine.
	if v, err := Eval("clamp(big * 10, 0, 100)", env); err != nil || v.(float64) != 100 {
		t.Errorf("clamp of overflow = %v, %v; want 100", v, err)
	}
	if v, err := Eval("safediv(1, nan, 42)", env); err != nil || v.(float64) != 42 {
		t.Errorf("safediv NaN divisor = %v, %v; want fallback 42", v, err)
	}
	if v, err := Eval("safediv(inf, 2, 42)", env); err != nil || v.(float64) != 42 {
		t.Errorf("safediv non-finite quotient = %v, %v; want fallback 42", v, err)
	}
	// A finite comparison against a non-finite operand is still a valid bool.
	if v, err := Eval("inf > 5", env); err != nil || v != true {
		t.Errorf("inf > 5 = %v, %v; want true", v, err)
	}
}

// TestNearlyEqual covers the tolerant ==/!= semantics on derived floats.
func TestNearlyEqual(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"0.1 + 0.2 == 0.3", true},
		{"0.1 + 0.2 != 0.3", false},
		{"1 == 1", true},
		{"1 == 1.01", false}, // a real cent of difference stays unequal
		{"100000000 == 100000001", false},
		{"(1/3) * 3 == 1", true},
	}
	for _, c := range cases {
		v, err := Eval(c.expr, Env{})
		if err != nil {
			t.Fatalf("Eval(%q): %v", c.expr, err)
		}
		if v.(bool) != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, v, c.want)
		}
	}
}

// TestChainedComparisonRejected: a < b < c is a parse error, not a silent
// bool-to-number coercion.
func TestChainedComparisonRejected(t *testing.T) {
	for _, expr := range []string{"5 < 3 < 10", "1 == 1 == 1", "a >= b > c"} {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) should reject chained comparison", expr)
		}
	}
	// Parenthesized coercion remains available for whoever really wants it.
	if v, err := Eval("(5 < 3) < 10", Env{}); err != nil || v != true {
		t.Errorf("(5<3)<10 = %v, %v; want true", v, err)
	}
}

// TestScientificNotation covers e/E exponent literals and the multi-dot error.
func TestScientificNotation(t *testing.T) {
	cases := map[string]float64{
		"1e3 + 1": 1001,
		"2.5E-2":  0.025,
		"1.5e+2":  150,
		".5e2":    50,
	}
	for expr, want := range cases {
		v, err := Eval(expr, Env{})
		if err != nil {
			t.Fatalf("Eval(%q): %v", expr, err)
		}
		if v.(float64) != want {
			t.Errorf("Eval(%q) = %v, want %g", expr, v, want)
		}
	}
	// "1e" and "2 e" don't swallow identifiers: the e stays a (unknown) ident.
	if _, err := Eval("1e", Env{}); err == nil {
		t.Error(`Eval("1e") should error (number then dangling ident)`)
	}
	// Multi-dot numbers fail at tokenize with the full bad token.
	if _, err := Tokenize("1.2.3"); err == nil {
		t.Error(`Tokenize("1.2.3") should error`)
	}
}

// TestParseLimits covers the nesting and length caps: pathological input is a
// returned error, never a stack overflow.
func TestParseLimits(t *testing.T) {
	deep := strings.Repeat("(", maxDepth+10) + "1" + strings.Repeat(")", maxDepth+10)
	if _, err := Parse(deep); err == nil {
		t.Error("over-deep parens should error")
	}
	unary := strings.Repeat("-", maxDepth+10) + "1"
	if _, err := Parse(unary); err == nil {
		t.Error("over-deep unary chain should error")
	}
	long := "1" + strings.Repeat("+1", MaxSourceLen)
	if _, err := Parse(long); err == nil {
		t.Error("over-long input should error")
	}
	// Real-world depth is untouched.
	ok := strings.Repeat("(", 300) + "1" + strings.Repeat("+1)", 300)
	if _, err := Parse(ok); err != nil {
		t.Errorf("depth 300 should parse: %v", err)
	}
}

// TestErrorPositions: parse errors expose their byte offset via errors.As.
func TestErrorPositions(t *testing.T) {
	_, err := Parse("1 + ^")
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("Parse error is %T, want *formula.Error", err)
	}
	if fe.Pos != 4 {
		t.Errorf("Pos = %d, want 4", fe.Pos)
	}
}

// TestCompileProgram covers the parse-once API: source round-trip, references,
// repeated evaluation, and concurrent use.
func TestCompileProgram(t *testing.T) {
	p, err := Compile("clamp(safediv(income - expense, income, 0) * 100, -100, 100)")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := p.References(); len(got) != 2 || got[0] != "expense" || got[1] != "income" {
		t.Errorf("References = %v, want [expense income]", got)
	}
	env := Env{Vars: map[string]float64{"income": 200, "expense": 100}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if v, err := p.Eval(env); err != nil || v.(float64) != 50 {
					t.Errorf("Program.Eval = %v, %v; want 50", v, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if _, err := Compile("1 +"); err == nil {
		t.Error("Compile of malformed input should error")
	}
}

// TestValidate covers save-path validation: parse check plus unknown-reference
// check with no evaluation.
func TestValidate(t *testing.T) {
	known := func(name string) bool { return name == "assets" || name == "liabilities" }
	if err := Validate("assets - liabilities", known); err != nil {
		t.Errorf("valid formula rejected: %v", err)
	}
	if err := Validate("assets - liablities", known); err == nil {
		t.Error("typo'd reference should fail validation")
	}
	if err := Validate("assets -", known); err == nil {
		t.Error("malformed formula should fail validation")
	}
	// nil known = parse-only.
	if err := Validate("anything_at_all + 1", nil); err != nil {
		t.Errorf("parse-only validation failed: %v", err)
	}
	// Validation never divides, so guards aren't needed.
	if err := Validate("assets / liabilities", known); err != nil {
		t.Errorf("validation must not evaluate: %v", err)
	}
}

// TestFunctionsListMatchesEvaluator: every documented function evaluates (no
// unknown-function error), so the docs can't drift ahead of the implementation.
func TestFunctionsListMatchesEvaluator(t *testing.T) {
	samples := map[string]string{
		"sum": "sum(1, 2)", "avg": "avg(1, 2)", "min": "min(1, 2)", "max": "max(1, 2)",
		"count": "count(1, 2)", "abs": "abs(-1)", "round": "round(1.5)", "floor": "floor(1.5)",
		"ceil": "ceil(1.5)", "clamp": "clamp(1, 0, 2)", "safediv": "safediv(1, 2, 0)",
		"if": "if(1, 2, 3)", "and": "and(1, 1)", "or": "or(0, 1)", "not": "not(0)",
		"contains": `contains("ab", "a")`, "lower": `lower("A")`,
	}
	fns := Functions()
	if len(fns) != len(samples) {
		t.Errorf("Functions() lists %d, test knows %d — update both together", len(fns), len(samples))
	}
	for _, f := range fns {
		expr, ok := samples[f.Name]
		if !ok {
			t.Errorf("Functions() lists %q but this test has no sample — add one", f.Name)
			continue
		}
		if _, err := Eval(expr, Env{}); err != nil {
			t.Errorf("documented function %q fails to evaluate: %v", f.Name, err)
		}
	}
}

// TestEvalCacheStable: repeated Eval of the same source (now AST-cached) stays
// correct and safe under concurrency.
func TestEvalCacheStable(t *testing.T) {
	env := Env{Vars: map[string]float64{"a": 1, "b": 2}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if v, err := Eval("a + b * 2", env); err != nil || v.(float64) != 5 {
					t.Errorf("cached Eval = %v, %v; want 5", v, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
