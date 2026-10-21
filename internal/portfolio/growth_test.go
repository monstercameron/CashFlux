// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestGrowthChangeFor(t *testing.T) {
	tests := []struct {
		name        string
		values      []money.Money
		wantDelta   int64
		wantPercent float64
		deltaKnown  bool
		pctKnown    bool
		wantReason  GrowthUnavailableReason
	}{
		{name: "ordinary growth", values: []money.Money{money.New(10000, "USD"), money.New(12500, "USD")}, wantDelta: 2500, wantPercent: 25, deltaKnown: true, pctKnown: true, wantReason: GrowthAvailable},
		{name: "loss from positive value", values: []money.Money{money.New(10000, "USD"), money.New(7500, "USD")}, wantDelta: -2500, wantPercent: -25, deltaKnown: true, pctKnown: true, wantReason: GrowthAvailable},
		{name: "negative starting value uses magnitude", values: []money.Money{money.New(-10000, "USD"), money.New(-7500, "USD")}, wantDelta: 2500, wantPercent: 25, deltaKnown: true, pctKnown: true, wantReason: GrowthAvailable},
		{name: "zero baseline has delta only", values: []money.Money{money.New(0, "USD"), money.New(100, "USD")}, wantDelta: 100, deltaKnown: true, wantReason: GrowthAvailable},
		{name: "flat series", values: []money.Money{money.New(42, "USD"), money.New(42, "USD")}, deltaKnown: true, pctKnown: true, wantReason: GrowthAvailable},
		{name: "too few values", values: []money.Money{money.New(42, "USD")}, wantReason: GrowthNeedsTwoValues},
		{name: "empty", wantReason: GrowthNeedsTwoValues},
		{name: "currency mismatch", values: []money.Money{money.New(1, "USD"), money.New(2, "EUR")}, wantReason: GrowthCurrencyMismatch},
		{name: "positive overflow", values: []money.Money{money.New(math.MinInt64, "USD"), money.New(math.MaxInt64, "USD")}, wantReason: GrowthDeltaOverflow},
		{name: "negative overflow", values: []money.Money{money.New(math.MaxInt64, "USD"), money.New(math.MinInt64, "USD")}, wantReason: GrowthDeltaOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GrowthChangeFor(tt.values)
			if got.DeltaKnown != tt.deltaKnown || got.PercentKnown != tt.pctKnown || got.Reason != tt.wantReason {
				t.Fatalf("availability = delta:%v percent:%v reason:%v, want delta:%v percent:%v reason:%v", got.DeltaKnown, got.PercentKnown, got.Reason, tt.deltaKnown, tt.pctKnown, tt.wantReason)
			}
			if tt.name == "too few values" && (!got.CurrentKnown || got.End.Amount != 42) {
				t.Errorf("single observation should retain its known current value: %+v", got)
			}
			if got.DeltaKnown && got.Delta.Amount != tt.wantDelta {
				t.Errorf("delta = %d, want %d", got.Delta.Amount, tt.wantDelta)
			}
			if got.PercentKnown && math.Abs(got.Percent-tt.wantPercent) > 1e-12 {
				t.Errorf("percent = %v, want %v", got.Percent, tt.wantPercent)
			}
		})
	}
}

func TestGrowthChangeForUsesEndpointsAndDoesNotMutate(t *testing.T) {
	values := []money.Money{money.New(100, "USD"), money.New(800, "EUR"), money.New(125, "USD")}
	want := append([]money.Money(nil), values...)
	got := GrowthChangeFor(values)
	if !got.DeltaKnown || got.Delta.Amount != 25 || !got.PercentKnown || got.Percent != 25 {
		t.Fatalf("GrowthChangeFor = %+v, want endpoint delta 25 and 25%%", got)
	}
	for i := range values {
		if values[i] != want[i] {
			t.Fatalf("input mutated at %d: got %v, want %v", i, values[i], want[i])
		}
	}
}

func BenchmarkGrowthChangeFor(b *testing.B) {
	values := make([]money.Money, 13)
	for i := range values {
		values[i] = money.New(int64(100000+i*1250), "USD")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GrowthChangeFor(values)
	}
}
