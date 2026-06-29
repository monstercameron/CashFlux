// SPDX-License-Identifier: MIT

package formula

import "testing"

func mustEvalNum(t *testing.T, expr string, vars map[string]float64) float64 {
	t.Helper()
	v, err := Eval(expr, Env{Vars: vars})
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", expr, err)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("Eval(%q) = %T, want float64", expr, v)
	}
	return f
}

func TestClamp(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"clamp(5, 0, 10)", 5},
		{"clamp(-3, 0, 10)", 0},
		{"clamp(99, 0, 10)", 10},
		{"clamp(5, 10, 0)", 5}, // lo/hi swapped
	}
	for _, c := range cases {
		if got := mustEvalNum(t, c.expr, nil); got != c.want {
			t.Errorf("%s = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestSafediv(t *testing.T) {
	if got := mustEvalNum(t, "safediv(10, 2, -1)", nil); got != 5 {
		t.Errorf("safediv(10,2,-1) = %v, want 5", got)
	}
	if got := mustEvalNum(t, "safediv(10, 0, -1)", nil); got != -1 {
		t.Errorf("safediv(10,0,-1) = %v, want -1 (fallback)", got)
	}
	// Real KPI shape: savings-rate guarded against zero income.
	vars := map[string]float64{"income": 0, "expense": 50}
	if got := mustEvalNum(t, "safediv(income - expense, income, 0) * 100", vars); got != 0 {
		t.Errorf("zero-income savings rate = %v, want 0", got)
	}
}

func TestFloorCeil(t *testing.T) {
	if got := mustEvalNum(t, "floor(2.9)", nil); got != 2 {
		t.Errorf("floor(2.9) = %v, want 2", got)
	}
	if got := mustEvalNum(t, "ceil(2.1)", nil); got != 3 {
		t.Errorf("ceil(2.1) = %v, want 3", got)
	}
	if got := mustEvalNum(t, "floor(-1.1)", nil); got != -2 {
		t.Errorf("floor(-1.1) = %v, want -2", got)
	}
}

func TestNewFuncsArityErrors(t *testing.T) {
	for _, expr := range []string{"clamp(1,2)", "safediv(1,2)", "floor(1,2)", "ceil()"} {
		if _, err := Eval(expr, Env{}); err == nil {
			t.Errorf("%s should error on wrong arity", expr)
		}
	}
}

func TestSavingsRateFormula(t *testing.T) {
	// The exact expression the savings KPI uses, against fundamental-derived vars.
	vars := map[string]float64{"income": 6982, "expense": 5322.67}
	got := mustEvalNum(t, "clamp(safediv(income - expense, income, 0) * 100, -100, 100)", vars)
	if got < 23.7 || got > 23.8 { // (6982-5322.67)/6982*100 ≈ 23.76%
		t.Errorf("savings rate = %v, want ~23.76", got)
	}
}
