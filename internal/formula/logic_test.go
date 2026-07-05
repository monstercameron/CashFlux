// SPDX-License-Identifier: MIT

package formula

import "testing"

// TestLogicFunctions covers and()/or()/not() — the language's logical
// combinators (it has no && / || operators).
func TestLogicFunctions(t *testing.T) {
	env := Env{
		Vars: map[string]float64{"a": 5, "b": 0},
		Strs: map[string]string{"payee": "Blue Bottle Coffee"},
	}
	cases := []struct {
		expr string
		want bool
	}{
		{`and(a > 1, b == 0)`, true},
		{`and(a > 1, b > 0)`, false},
		{`and(a > 1, b == 0, contains(payee, "coffee"))`, true},
		{`or(a > 10, b == 0)`, true},
		{`or(a > 10, b > 0)`, false},
		{`not(b > 0)`, true},
		{`not(a > 1)`, false},
		{`and(or(a > 10, a > 1), not(b > 0))`, true},
		{`and(a, b)`, false}, // numbers are truthy when non-zero
		{`or(a, b)`, true},
	}
	for _, c := range cases {
		v, err := Eval(c.expr, env)
		if err != nil {
			t.Errorf("Eval(%q): %v", c.expr, err)
			continue
		}
		b, ok := v.(bool)
		if !ok {
			t.Errorf("Eval(%q) = %T, want bool", c.expr, v)
			continue
		}
		if b != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, b, c.want)
		}
	}

	// Arity errors.
	for _, expr := range []string{`and(a > 1)`, `or(a > 1)`, `not(a, b)`} {
		if _, err := Eval(expr, env); err == nil {
			t.Errorf("Eval(%q) should error on arity", expr)
		}
	}
}
