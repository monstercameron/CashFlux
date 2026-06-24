// SPDX-License-Identifier: MIT

package formula

import "testing"

// TestEvalBoolCoercion covers asNumber's bool case: a comparison yields a bool,
// and using it in arithmetic coerces true→1 / false→0.
func TestEvalBoolCoercion(t *testing.T) {
	if got := evalNum(t, "(2 > 1) + 10", nil); got != 11 {
		t.Errorf("(2>1)+10 = %g, want 11 (true coerces to 1)", got)
	}
	if got := evalNum(t, "(1 > 2) + 10", nil); got != 10 {
		t.Errorf("(1>2)+10 = %g, want 10 (false coerces to 0)", got)
	}
}

// TestEvalUnaryPlus covers the unary "+" branch (parseUnary and eval's non-minus
// Unary case).
func TestEvalUnaryPlus(t *testing.T) {
	if got := evalNum(t, "+5", nil); got != 5 {
		t.Errorf("+5 = %g, want 5", got)
	}
	if got := evalNum(t, "+(3 - 1)", nil); got != 2 {
		t.Errorf("+(3-1) = %g, want 2", got)
	}
}

// TestEvalTruthyString covers truthy's string case via if().
func TestEvalTruthyString(t *testing.T) {
	if got := evalNum(t, `if("x", 1, 2)`, nil); got != 1 {
		t.Errorf(`if("x",1,2) = %g, want 1 (non-empty string is truthy)`, got)
	}
	if got := evalNum(t, `if("", 1, 2)`, nil); got != 2 {
		t.Errorf(`if("",1,2) = %g, want 2 (empty string is falsy)`, got)
	}
}

// TestEvalStringInequality covers the string "!=" branch of evalBinary.
func TestEvalStringInequality(t *testing.T) {
	v, err := Eval(`"a" != "b"`, Env{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if b, ok := v.(bool); !ok || !b {
		t.Errorf(`"a" != "b" = %v, want true`, v)
	}
}

// TestEvalMoreErrors covers the remaining error branches: comparing a string with
// a number, the empty-argument guards on avg/min/max, round's arity check, and a
// type error inside a numeric function's argument coercion.
func TestEvalMoreErrors(t *testing.T) {
	for _, in := range []string{
		`"a" == 1`,    // cannot compare string with number
		"avg()",       // needs at least one argument
		"min()",       // needs at least one argument
		"max()",       // needs at least one argument
		"round(1, 2)", // arity
		`sum("x")`,    // nums(): not a number
	} {
		if _, err := Eval(in, Env{}); err == nil {
			t.Errorf("Eval(%q) expected error", in)
		}
	}
}
