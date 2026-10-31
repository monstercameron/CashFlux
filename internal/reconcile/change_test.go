// SPDX-License-Identifier: MIT

package reconcile

import "testing"

func TestClassifyChange(t *testing.T) {
	tests := []struct {
		name            string
		current, target int64
		want            Shape
	}{
		{"same figure is no change", -800000, -800000, ShapeNone},
		{"owe less, still in debt", -800000, -750000, ShapeDebtFalls},
		{"owe more, still in debt", -800000, -850000, ShapeDebtRises},
		{"debt paid off to zero is not a debt move", -800000, 0, ShapeOther},
		{"debt paid off into credit", -800000, 50000, ShapeOther},
		{"credit to debt", 50000, -800000, ShapeOther},
		{"asset grows", 100000, 150000, ShapeOther},
		{"asset shrinks into negative", 100000, -150000, ShapeOther},
		{"overdrawn account owes less", -150000, -100000, ShapeDebtFalls},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyChange(tt.current, tt.target); got != tt.want {
				t.Errorf("ClassifyChange(%d, %d) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}

func TestLooksLikeUnsignedDebt(t *testing.T) {
	tests := []struct {
		name           string
		current, typed int64
		want           bool
	}{
		{"debt typed without a minus sign", -800000, 750000, true},
		{"debt typed with a minus sign", -800000, -750000, false},
		{"debt paid off to zero", -800000, 0, false},
		{"debt paid into credit on purpose", -800000, 50000, true},
		{"asset typed positive", 100000, 150000, false},
		{"asset typed negative", 100000, -150000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LooksLikeUnsignedDebt(tt.current, tt.typed); got != tt.want {
				t.Errorf("LooksLikeUnsignedDebt(%d, %d) = %v, want %v", tt.current, tt.typed, got, tt.want)
			}
		})
	}
}
