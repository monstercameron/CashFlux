// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRolloverChainCapsCarryIn(t *testing.T) {
	const limit int64 = 20000
	tests := []struct {
		name          string
		carry         int64
		capPeriods    int
		wantCarryIn   int64
		wantEffective int64
	}{
		{"uncapped surplus passes through", 90000, 0, 90000, 110000},
		{"negative cap is uncapped", 90000, -1, 90000, 110000},
		{"within cap unchanged", 15000, 1, 15000, 35000},
		{"at cap unchanged", 20000, 1, 20000, 40000},
		{"surplus clamped to one period", 90000, 1, 20000, 40000},
		{"surplus clamped to two periods", 90000, 2, 40000, 60000},
		{"deficit never clamped", -5000, 1, -5000, 15000},
		{"zero carry unchanged", 0, 1, 0, 20000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := money.New(limit, "USD")
			priorSpend := money.New(limit-tc.carry, "USD")
			states, err := BuildRolloverChain([]RolloverPeriod{
				{Start: mustDate("2026-08-01"), Base: base, Spend: priorSpend},
				{Start: mustDate("2026-09-01"), Base: base, Spend: money.Zero("USD")},
			}, tc.capPeriods)
			if err != nil {
				t.Fatalf("BuildRolloverChain: %v", err)
			}
			if got := states[1].CarryIn.Amount; got != tc.wantCarryIn {
				t.Errorf("carry-in = %d, want %d", got, tc.wantCarryIn)
			}
			if got := states[1].EffectiveCap.Amount; got != tc.wantEffective {
				t.Errorf("effective cap = %d, want %d", got, tc.wantEffective)
			}
		})
	}
}
