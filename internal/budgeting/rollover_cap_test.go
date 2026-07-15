// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCapCarryover(t *testing.T) {
	limit := money.New(20000, "USD") // $200 monthly limit
	tests := []struct {
		name       string
		carry      int64
		capPeriods int
		want       int64
	}{
		{"uncapped passes surplus through", 90000, 0, 90000},
		{"negative cap is uncapped", 90000, -1, 90000},
		{"within cap unchanged", 15000, 1, 15000},
		{"at cap unchanged", 20000, 1, 20000},
		{"surplus clamped to 1x", 90000, 1, 20000},
		{"surplus clamped to 2x", 90000, 2, 40000},
		{"deficit never clamped", -5000, 1, -5000},
		{"zero carry unchanged", 0, 1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CapCarryover(money.New(tc.carry, "USD"), limit, tc.capPeriods)
			if got.Amount != tc.want {
				t.Errorf("CapCarryover(%d, cap=%d) = %d, want %d", tc.carry, tc.capPeriods, got.Amount, tc.want)
			}
		})
	}
}

func TestCappedCarryover(t *testing.T) {
	limit := money.New(20000, "USD")

	t.Run("clamps surplus then adds this period's limit", func(t *testing.T) {
		// $900 surplus capped at 1x ($200) + $200 limit = $400 available.
		got, err := CappedCarryover(money.New(90000, "USD"), limit, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Amount != 40000 {
			t.Errorf("available = %d, want 40000", got.Amount)
		}
	})

	t.Run("uncapped matches plain Carryover", func(t *testing.T) {
		got, err := CappedCarryover(money.New(90000, "USD"), limit, 0)
		if err != nil {
			t.Fatal(err)
		}
		plain, _ := Carryover(money.New(90000, "USD"), limit)
		if got.Amount != plain.Amount {
			t.Errorf("uncapped = %d, want %d (plain Carryover)", got.Amount, plain.Amount)
		}
	})

	t.Run("deficit carries in full", func(t *testing.T) {
		// -$50 debt + $200 limit = $150 available; cap doesn't touch the deficit.
		got, err := CappedCarryover(money.New(-5000, "USD"), limit, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Amount != 15000 {
			t.Errorf("available = %d, want 15000", got.Amount)
		}
	})
}
