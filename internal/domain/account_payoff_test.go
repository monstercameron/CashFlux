package domain

import "testing"

func boolp(b bool) *bool { return &b }

func TestIncludedInPayoff(t *testing.T) {
	tests := []struct {
		name string
		acc  Account
		want bool
	}{
		{"mortgage excluded by default", Account{Type: TypeMortgage}, false},
		{"credit card included by default", Account{Type: TypeCreditCard}, true},
		{"loan included by default", Account{Type: TypeLoan}, true},
		{"user opts the mortgage in", Account{Type: TypeMortgage, IncludeInPayoff: boolp(true)}, true},
		{"user opts a card out", Account{Type: TypeCreditCard, IncludeInPayoff: boolp(false)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.acc.IncludedInPayoff(); got != tc.want {
				t.Errorf("IncludedInPayoff() = %v, want %v", got, tc.want)
			}
		})
	}
}
