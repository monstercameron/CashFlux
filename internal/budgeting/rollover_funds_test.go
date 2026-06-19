package budgeting

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCarryover(t *testing.T) {
	tests := []struct {
		name          string
		prevRemaining money.Money
		limit         money.Money
		want          int64
	}{
		{"unspent rolls forward", money.New(3000, "USD"), money.New(10000, "USD"), 13000},
		{"overspend carries as debt", money.New(-2000, "USD"), money.New(10000, "USD"), 8000},
		{"exactly spent", money.New(0, "USD"), money.New(10000, "USD"), 10000},
		{"deep overdraw goes negative", money.New(-15000, "USD"), money.New(10000, "USD"), -5000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Carryover(tc.prevRemaining, tc.limit)
			if err != nil {
				t.Fatalf("Carryover: %v", err)
			}
			if got.Amount != tc.want || got.Currency != "USD" {
				t.Errorf("Carryover = %v, want %d USD", got, tc.want)
			}
		})
	}
}

func TestCarryoverCurrencyMismatch(t *testing.T) {
	if _, err := Carryover(money.New(100, "USD"), money.New(100, "EUR")); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("err = %v, want ErrCurrencyMismatch", err)
	}
}

func TestSinkingFundContribution(t *testing.T) {
	tests := []struct {
		name    string
		target  int64
		periods int
		want    int64
	}{
		{"even split", 120000, 12, 10000},
		{"ceils the remainder", 100000, 12, 8334}, // 8333.33 → round up so 12×8334 ≥ target
		{"single period", 5000, 1, 5000},
		{"zero periods funds all now", 5000, 0, 5000},
		{"negative periods funds all now", 5000, -3, 5000},
		{"zero target", 0, 12, 0},
		{"negative target", -100, 12, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SinkingFundContribution(money.New(tc.target, "USD"), tc.periods)
			if got.Amount != tc.want {
				t.Errorf("SinkingFundContribution(%d, %d) = %d, want %d", tc.target, tc.periods, got.Amount, tc.want)
			}
		})
	}
}

func TestSinkingFundContributionReachesTarget(t *testing.T) {
	// The ceiling guarantee: periods × contribution must always cover the target.
	cases := []struct {
		target  int64
		periods int
	}{{100000, 12}, {99999, 7}, {1, 3}, {77777, 13}}
	for _, c := range cases {
		per := SinkingFundContribution(money.New(c.target, "USD"), c.periods)
		if per.Amount*int64(c.periods) < c.target {
			t.Errorf("target %d over %d periods: %d×%d = %d < target", c.target, c.periods, per.Amount, c.periods, per.Amount*int64(c.periods))
		}
	}
}

func TestSinkingFundAccrued(t *testing.T) {
	target := money.New(120000, "USD")
	contribution := money.New(10000, "USD")
	tests := []struct {
		made int
		want int64
	}{
		{0, 0},
		{-1, 0},
		{3, 30000},
		{12, 120000},
		{15, 120000}, // capped at target, never overshoots
	}
	for _, tc := range tests {
		got, err := SinkingFundAccrued(contribution, target, tc.made)
		if err != nil {
			t.Fatalf("SinkingFundAccrued(made=%d): %v", tc.made, err)
		}
		if got.Amount != tc.want {
			t.Errorf("SinkingFundAccrued(made=%d) = %d, want %d", tc.made, got.Amount, tc.want)
		}
	}
}

func TestSinkingFundAccruedErrors(t *testing.T) {
	if _, err := SinkingFundAccrued(money.New(100, "USD"), money.New(100, "EUR"), 1); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("currency mismatch err = %v", err)
	}
	huge := money.New(1<<62, "USD")
	if _, err := SinkingFundAccrued(huge, huge, 4); !errors.Is(err, money.ErrOverflow) {
		t.Errorf("overflow err = %v, want ErrOverflow", err)
	}
}

func TestSinkingFundProgress(t *testing.T) {
	tests := []struct {
		accrued, target int64
		want            int
	}{
		{0, 120000, 0},
		{30000, 120000, 25},
		{120000, 120000, 100},
		{200000, 120000, 100}, // capped
		{50, 0, 100},          // non-positive target, something saved
		{0, 0, 0},             // non-positive target, nothing saved
	}
	for _, tc := range tests {
		got := SinkingFundProgress(money.New(tc.accrued, "USD"), money.New(tc.target, "USD"))
		if got != tc.want {
			t.Errorf("SinkingFundProgress(%d/%d) = %d, want %d", tc.accrued, tc.target, got, tc.want)
		}
	}
}
