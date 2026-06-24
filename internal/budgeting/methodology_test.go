// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func TestMethodologyValid(t *testing.T) {
	for _, m := range []Methodology{MethodSimple, MethodZeroBased, MethodEnvelope} {
		if !m.Valid() {
			t.Errorf("%q should be valid", m)
		}
	}
	if Methodology("nonsense").Valid() {
		t.Error(`"nonsense" should be invalid`)
	}
}

func TestParseMethodology(t *testing.T) {
	cases := map[string]Methodology{
		"":           MethodSimple, // unset → safe default
		"simple":     MethodSimple,
		"zero-based": MethodZeroBased,
		"envelope":   MethodEnvelope,
		"bogus":      MethodSimple, // unknown → safe default
	}
	for in, want := range cases {
		if got := ParseMethodology(in); got != want {
			t.Errorf("ParseMethodology(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToAssign(t *testing.T) {
	tests := []struct {
		name                  string
		income, totalBudgeted int64
		want                  int64
	}{
		{"under-assigned leaves a remainder", 500000, 300000, 200000},
		{"fully assigned is zero", 400000, 400000, 0},
		{"over-assigned is negative", 300000, 450000, -150000},
		{"no income", 0, 100000, -100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToAssign(tt.income, tt.totalBudgeted); got != tt.want {
				t.Errorf("ToAssign(%d, %d) = %d, want %d", tt.income, tt.totalBudgeted, got, tt.want)
			}
		})
	}
}
