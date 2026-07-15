// SPDX-License-Identifier: MIT

package amountmath

import (
	"math"
	"testing"
)

func TestEvalAmount(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   float64
		wantOK bool
	}{
		{"plain integer passes through", "12", 0, false},
		{"plain decimal passes through", "45.99", 0, false},
		{"plain with commas passes through", "1,234.56", 0, false},
		{"whitespace plain passes through", "  45.99  ", 0, false},
		{"empty is not a formula", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"leading minus alone is not a formula", "-5", 0, false},

		{"multiplication", "45.99*3", 137.97, true},
		{"division", "120/4", 30, true},
		{"grouped", "(12+8)*2", 40, true},
		{"addition", "10+5.5", 15.5, true},
		{"subtraction positive", "20-8", 12, true},
		{"commas stripped in formula", "1,234*2", 2468, true},

		{"negative result rejected", "10-20", 0, false},
		{"junk rejected", "abc", 0, false},
		{"trailing operator rejected", "5*", 0, false},
		{"division by zero rejected", "5/0", 0, false},
		{"unknown variable rejected", "x*2", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := EvalAmount(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("EvalAmount(%q) ok = %v, want %v (got %v)", tt.in, ok, tt.wantOK, got)
			}
			if ok && math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("EvalAmount(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
