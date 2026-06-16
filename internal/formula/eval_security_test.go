package formula

import (
	"strings"
	"testing"
)

// TestEvalSandboxRejectsNonAllowlistedFunctions is the core sandbox guarantee:
// only the curated function set runs. Anything that looks like a host or
// arbitrary function is an "unknown function" error — there is no escape hatch.
func TestEvalSandboxRejectsNonAllowlistedFunctions(t *testing.T) {
	blocked := []string{
		"exec(1)", "eval(1)", "system(1)", "open(1)", "read(1)", "write(1)",
		"import(1)", "require(1)", "fetch(1)", "panic(1)", "go(1)", "make(1)",
		"println(1)", "printf(1)", "os(1)", "fmt(1)", "reflect(1)",
		// The allow-list is case-sensitive: only the exact lowercase names resolve.
		"SUM(1)", "Sum(1)", "ABS(1)", "Round(1)", "IF(1,2,3)",
	}
	for _, in := range blocked {
		if v, err := Eval(in, Env{}); err == nil {
			t.Errorf("Eval(%q) = %v, expected an unknown-function error (sandbox escape!)", in, v)
		}
	}
}

// TestEvalUnknownVariablesError confirms a formula can only read the variables it
// was handed — an undeclared name never silently resolves to zero.
func TestEvalUnknownVariablesError(t *testing.T) {
	env := Env{Vars: map[string]float64{"income": 100}}
	if _, err := Eval("income + secret", env); err == nil {
		t.Error("Eval referencing an undeclared variable should error")
	}
	if got, err := Eval("income", env); err != nil || got.(float64) != 100 {
		t.Errorf("declared variable = %v, %v; want 100", got, err)
	}
}

// TestEvalOnlyProducesScalarValues asserts evaluation can only yield a number,
// string, or bool — never any host type — so a formula can't smuggle out a value
// of another kind.
func TestEvalOnlyProducesScalarValues(t *testing.T) {
	for _, in := range []string{
		"1 + 1", `"hi"`, "2 > 1", `if(1, "a", "b")`, "sum(1, 2)", "-3", "min(1, 2) == 1", "abs(-2) % 3",
	} {
		v, err := Eval(in, Env{})
		if err != nil {
			t.Fatalf("Eval(%q): %v", in, err)
		}
		switch v.(type) {
		case float64, string, bool:
		default:
			t.Errorf("Eval(%q) produced %T, want float64/string/bool", in, v)
		}
	}
}

// TestEvalDeepNesting ensures deeply nested expressions evaluate correctly without
// crashing at a reasonable depth.
func TestEvalDeepNesting(t *testing.T) {
	const depth = 300
	expr := strings.Repeat("(", depth) + "1" + strings.Repeat("+1)", depth)
	v, err := Eval(expr, Env{})
	if err != nil {
		t.Fatalf("deep nest: %v", err)
	}
	if v.(float64) != float64(depth+1) {
		t.Errorf("deep nest = %v, want %d", v, depth+1)
	}
}

// TestEvalDeterministic confirms the same expression yields the same result every
// time (no hidden state/clock/randomness in the engine).
func TestEvalDeterministic(t *testing.T) {
	const in = "round(avg(income, expense) * 1.5) + min(income, expense)"
	env := Env{Vars: map[string]float64{"income": 4200, "expense": 2600}}
	first, err := Eval(in, env)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := Eval(in, env)
		if err != nil || got != first {
			t.Fatalf("run %d = %v, %v; want stable %v", i, got, err, first)
		}
	}
}

// TestEvalNumericEdgeCases covers signs, modulo with negatives, and chained unary.
func TestEvalNumericEdgeCases(t *testing.T) {
	cases := map[string]float64{
		"--5":        5,   // chained unary minus
		"-(-5)":      5,   // parenthesized
		"+7":         7,   // unary plus
		"-7 % 3":     -1,  // Go math.Mod keeps the dividend's sign
		"10 / 4":     2.5, // float division, not integer
		"round(2.5)": 3,   // round-half-away-from-zero (math.Round)
		"round(3.5)": 4,
		"abs(-0)":    0,
	}
	for in, want := range cases {
		v, err := Eval(in, Env{})
		if err != nil {
			t.Fatalf("Eval(%q): %v", in, err)
		}
		if v.(float64) != want {
			t.Errorf("Eval(%q) = %v, want %g", in, v, want)
		}
	}
}

// TestEvalMalformedInputsError makes sure malformed expressions return an error
// rather than panicking.
func TestEvalMalformedInputsError(t *testing.T) {
	for _, in := range []string{
		"", "   ", "1 +", "(1", "1 2", ")", "sum(a,)", "* 3", "1 +* 2",
	} {
		if v, err := Eval(in, Env{}); err == nil {
			t.Errorf("Eval(%q) = %v, expected error", in, v)
		}
	}
}
