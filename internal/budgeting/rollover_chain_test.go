// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRolloverChainUsesPriorEffectiveCap(t *testing.T) {
	starts := []time.Time{
		time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC),
	}
	periods := []RolloverPeriod{
		{Start: starts[0], Base: money.New(18564, "USD"), Spend: money.New(61608, "USD")}, // Sep: $430.44 over.
		{Start: starts[1], Base: money.New(63564, "USD"), Spend: money.New(24250, "USD")}, // Oct effective cap $205.20; spend $242.50.
		{Start: starts[2], Base: money.New(45000, "USD"), Spend: money.Zero("USD")},
	}
	chain, err := BuildRolloverChain(periods, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []RolloverPeriodState{
		{Start: starts[0], Base: money.New(18564, "USD"), CarryIn: money.Zero("USD"), EffectiveCap: money.New(18564, "USD"), Spend: money.New(61608, "USD"), CarryOut: money.New(-43044, "USD")},
		{Start: starts[1], Base: money.New(63564, "USD"), CarryIn: money.New(-43044, "USD"), EffectiveCap: money.New(20520, "USD"), Spend: money.New(24250, "USD"), CarryOut: money.New(-3730, "USD")},
		{Start: starts[2], Base: money.New(45000, "USD"), CarryIn: money.New(-3730, "USD"), EffectiveCap: money.New(41270, "USD"), Spend: money.Zero("USD"), CarryOut: money.New(41270, "USD")},
	}
	if len(chain) != len(want) {
		t.Fatalf("chain length = %d, want %d", len(chain), len(want))
	}
	for i := range want {
		if chain[i] != want[i] {
			t.Errorf("period %s = %+v, want %+v", starts[i].Format("Jan"), chain[i], want[i])
		}
	}
}

func TestRolloverCarryCapAppliesToNextPeriodCarryIn(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	chain, err := BuildRolloverChain([]RolloverPeriod{
		{Start: start, Base: money.New(30000, "USD"), Spend: money.Zero("USD")},
		{Start: start.AddDate(0, 1, 0), Base: money.New(12000, "USD"), Spend: money.Zero("USD")},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if chain[0].CarryOut.Amount != 30000 {
		t.Errorf("first carry-out = %d, want the exact $300 surplus", chain[0].CarryOut.Amount)
	}
	if chain[1].CarryIn.Amount != 12000 || chain[1].EffectiveCap.Amount != 24000 {
		t.Errorf("next carry-in/cap = %d/%d, want next period's 1× cap $120/$240", chain[1].CarryIn.Amount, chain[1].EffectiveCap.Amount)
	}
}
