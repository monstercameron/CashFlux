// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"

	"github.com/monstercameron/CashFlux/internal/money"
)

// GrowthUnavailableReason explains why a growth fact could not be computed.
type GrowthUnavailableReason uint8

const (
	// GrowthAvailable means the delta is known; the percentage may still be
	// unavailable when the starting value is zero.
	GrowthAvailable GrowthUnavailableReason = iota
	// GrowthNeedsTwoValues means the series contains fewer than two valuations.
	GrowthNeedsTwoValues
	// GrowthCurrencyMismatch means endpoints use different currencies.
	GrowthCurrencyMismatch
	// GrowthDeltaOverflow means the endpoint difference does not fit in minor units.
	GrowthDeltaOverflow
)

// GrowthChange contains the checked change between the first and last values in
// a series. Delta and Percent have independent availability because a zero
// starting value still permits an absolute change but has no meaningful rate.
type GrowthChange struct {
	Start        money.Money
	End          money.Money
	Delta        money.Money
	Percent      float64
	CurrentKnown bool
	DeltaKnown   bool
	PercentKnown bool
	Reason       GrowthUnavailableReason
}

// GrowthChangeFor returns the absolute and relative change from the first to
// the last series value. It uses checked money subtraction and never mutates
// the input slice. The percentage uses the magnitude of the starting value as
// its denominator, matching the investments chart's historical convention.
func GrowthChangeFor(values []money.Money) GrowthChange {
	if len(values) == 0 {
		return GrowthChange{Reason: GrowthNeedsTwoValues}
	}
	start, end := values[0], values[len(values)-1]
	change := GrowthChange{Start: start, End: end, CurrentKnown: true}
	if len(values) < 2 {
		change.Reason = GrowthNeedsTwoValues
		return change
	}
	if start.Currency != end.Currency {
		change.Reason = GrowthCurrencyMismatch
		return change
	}
	delta, err := end.Sub(start)
	if err != nil {
		change.Reason = GrowthDeltaOverflow
		return change
	}
	change.Delta, change.DeltaKnown, change.Reason = delta, true, GrowthAvailable
	if start.Amount == 0 {
		return change
	}
	pct := float64(delta.Amount) / math.Abs(float64(start.Amount)) * 100
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return change
	}
	change.Percent, change.PercentKnown = pct, true
	return change
}
