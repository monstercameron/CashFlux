// SPDX-License-Identifier: MIT

package payoff

import "testing"

func TestMinimumViablePayment(t *testing.T) {
	tests := []struct {
		name    string
		balance int64
		apr     float64
		want    int64
	}{
		{"zero balance", 0, 19.99, 0},
		{"negative balance", -100, 19.99, 0},
		{"zero apr needs a single unit", 100000, 0, 1},
		// 12% APR on $1000 → 1% monthly = $10.00 interest → min viable $10.01.
		{"twelve percent", 100000, 12, 1001},
		// 24% APR on $5000 → 2% monthly = $100.00 → min viable $100.01.
		{"twentyfour percent", 500000, 24, 10001},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MinimumViablePayment(tc.balance, tc.apr); got != tc.want {
				t.Errorf("MinimumViablePayment(%d, %v) = %d, want %d", tc.balance, tc.apr, got, tc.want)
			}
		})
	}
}

func TestMinimumViablePaymentActuallyClears(t *testing.T) {
	// The minimum viable payment must make Project succeed; one unit less must not.
	balance, apr := int64(250000), 18.0
	min := MinimumViablePayment(balance, apr)
	if _, ok := Project(balance, apr, min); !ok {
		t.Errorf("min viable payment %d should clear the debt", min)
	}
	if _, ok := Project(balance, apr, min-1); ok {
		t.Errorf("one minor unit below min (%d) should NOT clear the debt", min-1)
	}
}
