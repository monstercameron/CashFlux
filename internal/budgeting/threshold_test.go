package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

// TestClassifyBoundaries pins the exact threshold behavior (==limit is over,
// ==near% is near) so a future tweak to the comparisons can't silently shift a
// budget's state off by one cent (D2).
func TestClassifyBoundaries(t *testing.T) {
	const limit = 10000 // $100.00
	usd := func(a int64) money.Money { return money.New(a, "USD") }
	tests := []struct {
		name  string
		spent int64
		near  float64
		want  State
	}{
		{"well under is OK", 5000, 0.8, StateOK},
		{"one cent below near is OK", 7999, 0.8, StateOK},
		{"exactly at near% is Near", 8000, 0.8, StateNear},
		{"between near and limit is Near", 9999, 0.8, StateNear},
		{"exactly at the limit is Over", 10000, 0.8, StateOver},
		{"above the limit is Over", 12000, 0.8, StateOver},
		{"zero spend is OK", 0, 0.8, StateOK},
		{"different threshold: at 90% is Near", 9000, 0.9, StateNear},
		{"different threshold: one cent below 90% is OK", 8999, 0.9, StateOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(usd(tt.spent), usd(limit), tt.near); got != tt.want {
				t.Errorf("classify(spent=%d, limit=%d, near=%.2f) = %q, want %q", tt.spent, limit, tt.near, got, tt.want)
			}
		})
	}
}

// TestClassifyZeroLimit covers the degenerate "no limit set" budget: any spend
// is over, no spend is OK.
func TestClassifyZeroLimit(t *testing.T) {
	usd := func(a int64) money.Money { return money.New(a, "USD") }
	if got := classify(usd(1), usd(0), 0.8); got != StateOver {
		t.Errorf("zero limit + spend = %q, want Over", got)
	}
	if got := classify(usd(0), usd(0), 0.8); got != StateOK {
		t.Errorf("zero limit + no spend = %q, want OK", got)
	}
}

// TestPercentBoundaries covers the percent helper, including the zero-limit
// guard (no divide-by-zero; any spend reads as 100%).
func TestPercentBoundaries(t *testing.T) {
	usd := func(a int64) money.Money { return money.New(a, "USD") }
	cases := []struct {
		spent, limit int64
		want         int
	}{
		{5000, 10000, 50},
		{10000, 10000, 100},
		{12000, 10000, 120},
		{0, 10000, 0},
		{500, 0, 100}, // zero limit + spend → 100, not a panic
		{0, 0, 0},     // zero limit + no spend → 0
	}
	for _, c := range cases {
		if got := percent(usd(c.spent), usd(c.limit)); got != c.want {
			t.Errorf("percent(%d, %d) = %d, want %d", c.spent, c.limit, got, c.want)
		}
	}
}
