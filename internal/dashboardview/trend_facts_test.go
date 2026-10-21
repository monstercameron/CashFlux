// SPDX-License-Identifier: MIT

package dashboardview

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareTrendFactsCheckedDeltaAndMagnitude(t *testing.T) {
	tests := []struct {
		name      string
		series    []money.Money
		live      money.Money
		available bool
		up        bool
		delta     int64
		magnitude int64
		err       error
	}{
		{name: "up", series: []money.Money{money.New(100, "USD"), money.New(175, "USD")}, live: money.New(175, "USD"), available: true, up: true, delta: 75, magnitude: 75},
		{name: "down", series: []money.Money{money.New(175, "USD"), money.New(100, "USD")}, live: money.New(100, "USD"), available: true, delta: -75, magnitude: 75},
		{name: "zero", series: []money.Money{money.New(100, "USD"), money.New(100, "USD")}, live: money.New(100, "USD"), available: true},
		{name: "positive overflow", series: []money.Money{money.New(math.MinInt64, "USD"), money.New(math.MaxInt64, "USD")}, live: money.New(math.MaxInt64, "USD"), err: money.ErrOverflow},
		{name: "negative overflow", series: []money.Money{money.New(math.MaxInt64, "USD"), money.New(math.MinInt64, "USD")}, live: money.New(math.MinInt64, "USD"), err: money.ErrOverflow},
		{name: "minimum magnitude overflow", series: []money.Money{money.Zero("USD"), money.New(math.MinInt64, "USD")}, live: money.New(math.MinInt64, "USD"), delta: math.MinInt64, err: money.ErrOverflow},
		{name: "empty", live: money.Zero("USD"), err: errEmptyTrend},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := PrepareTrendFacts(test.series, test.live)
			if got.DeltaAvailable != test.available || got.DeltaUp != test.up || got.Delta.Amount != test.delta || got.DeltaMagnitude.Amount != test.magnitude {
				t.Fatalf("delta facts = %+v", got)
			}
			if !errors.Is(got.DeltaError, test.err) {
				t.Fatalf("delta error = %v, want %v", got.DeltaError, test.err)
			}
			if test.name == "empty" {
				if got.RangeAvailable {
					t.Fatal("empty series produced a range")
				}
				return
			}
			if !got.RangeAvailable {
				t.Fatalf("valid range unavailable: %v", got.RangeError)
			}
		})
	}
}

func TestPrepareTrendFactsIncludesLivePointInRange(t *testing.T) {
	got := PrepareTrendFacts([]money.Money{money.New(20, "USD"), money.New(40, "USD"), money.New(30, "USD")}, money.New(10, "USD"))
	if !got.RangeAvailable || got.Low.Amount != 10 || got.High.Amount != 40 {
		t.Fatalf("range = [%d,%d], available=%v", got.Low.Amount, got.High.Amount, got.RangeAvailable)
	}
}

func TestPrepareTrendFactsRejectsMixedCurrencyRange(t *testing.T) {
	got := PrepareTrendFacts([]money.Money{money.New(20, "USD"), money.New(40, "EUR")}, money.New(30, "USD"))
	if got.RangeAvailable || got.DeltaAvailable || !errors.Is(got.RangeError, money.ErrCurrencyMismatch) {
		t.Fatalf("mixed currency facts = %+v", got)
	}
}
