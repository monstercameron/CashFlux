package payoff

import "testing"

func TestSuggestedExtra(t *testing.T) {
	tests := []struct {
		name  string
		debts []Debt
		want  int64
	}{
		{
			name:  "a quarter of the total minimum payments",
			debts: []Debt{{Balance: 200000, MinPayment: 5000}, {Balance: 50000, MinPayment: 2000}},
			want:  1750, // (5000+2000)/4
		},
		{
			name:  "falls back to 1% of balance when minimums are unknown",
			debts: []Debt{{Balance: 100000, MinPayment: 0}},
			want:  1000,
		},
		{
			name:  "floors at one minor unit for a tiny balance",
			debts: []Debt{{Balance: 50, MinPayment: 0}},
			want:  1,
		},
		{
			name:  "no debts suggests zero",
			debts: nil,
			want:  0,
		},
		{
			name:  "cleared debts are ignored",
			debts: []Debt{{Balance: 0, MinPayment: 9999}, {Balance: 40000, MinPayment: 4000}},
			want:  1000, // only the active debt's 4000/4
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SuggestedExtra(tc.debts); got != tc.want {
				t.Errorf("SuggestedExtra = %d, want %d", got, tc.want)
			}
		})
	}
}
