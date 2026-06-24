// SPDX-License-Identifier: MIT

package formula

import "testing"

func evalNum(t *testing.T, in string, vars map[string]float64) float64 {
	t.Helper()
	v, err := Eval(in, Env{Vars: vars})
	if err != nil {
		t.Fatalf("Eval(%q): %v", in, err)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("Eval(%q) = %v (%T), want number", in, v, v)
	}
	return f
}

func TestEvalArithmetic(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"10 / 2 - 1", 4},
		{"7 % 3", 1},
		{"-5 + 2", -3},
		{"2 + 2 == 4", 1}, // bool true coerces to 1 only via asNumber; here compare returns bool, checked below
	}
	for _, tt := range tests[:5] {
		if got := evalNum(t, tt.in, nil); got != tt.want {
			t.Errorf("Eval(%q) = %g, want %g", tt.in, got, tt.want)
		}
	}
}

func TestEvalComparisons(t *testing.T) {
	cases := map[string]bool{
		"2 > 1":      true,
		"2 < 1":      false,
		"3 >= 3":     true,
		"3 != 4":     true,
		`"a" == "a"`: true,
		`"a" == "b"`: false,
		"1 + 1 == 2": true,
	}
	for in, want := range cases {
		v, err := Eval(in, Env{})
		if err != nil {
			t.Fatalf("Eval(%q): %v", in, err)
		}
		b, ok := v.(bool)
		if !ok {
			t.Fatalf("Eval(%q) = %v (%T), want bool", in, v, v)
		}
		if b != want {
			t.Errorf("Eval(%q) = %v, want %v", in, b, want)
		}
	}
}

func TestEvalFunctions(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"sum(1, 2, 3, 4)", 10},
		{"avg(2, 4, 6)", 4},
		{"min(5, 2, 8)", 2},
		{"max(5, 2, 8)", 8},
		{"count(1, 2, 3)", 3},
		{"abs(-7)", 7},
		{"round(2.6)", 3},
		{"if(1 > 0, 10, 20)", 10},
		{"if(0, 10, 20)", 20},
		{"round(avg(1, 2) * 4)", 6},
	}
	for _, tt := range tests {
		if got := evalNum(t, tt.in, nil); got != tt.want {
			t.Errorf("Eval(%q) = %g, want %g", tt.in, got, tt.want)
		}
	}
}

func TestEvalVariables(t *testing.T) {
	vars := map[string]float64{"income": 5000, "expense": 1800}
	if got := evalNum(t, "income - expense", vars); got != 3200 {
		t.Errorf("income - expense = %g, want 3200", got)
	}
	if got := evalNum(t, "round((income - expense) / income * 100)", vars); got != 64 {
		t.Errorf("savings rate = %g, want 64", got)
	}
}

func TestEvalErrors(t *testing.T) {
	for _, in := range []string{
		"1 / 0",
		"5 % 0",
		"unknownVar + 1",
		"nope(1)",
		"abs(1, 2)",
		"if(1, 2)",
		`"a" + 1`,
	} {
		if _, err := Eval(in, Env{}); err == nil {
			t.Errorf("Eval(%q) expected error", in)
		}
	}
}
