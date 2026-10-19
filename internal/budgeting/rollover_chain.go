// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// RolloverPeriod is one chronological input period for a rollover calculation.
type RolloverPeriod struct {
	Start time.Time
	Base  money.Money
	Spend money.Money
}

// RolloverPeriodState exposes every amount needed to explain one link in a
// rollover chain.
type RolloverPeriodState struct {
	Start        time.Time
	Base         money.Money
	CarryIn      money.Money
	EffectiveCap money.Money
	Spend        money.Money
	CarryOut     money.Money
}

// BuildRolloverChain computes carry as prior effective cap minus prior spend.
// Inputs must be in chronological order and use the same currency. A positive
// capPeriods limits surplus carry-in to that many current base limits; debt is
// retained. CarryOut always reports effective cap minus spend before next-period
// clamping, so the chain remains auditable.
func BuildRolloverChain(periods []RolloverPeriod, capPeriods int) ([]RolloverPeriodState, error) {
	out := make([]RolloverPeriodState, 0, len(periods))
	var carry money.Money
	for i, period := range periods {
		if i > 0 && !period.Start.After(periods[i-1].Start) {
			return nil, fmt.Errorf("rollover period %d is not after the prior period", i)
		}
		if period.Base.Currency != period.Spend.Currency {
			return nil, fmt.Errorf("rollover period %s: %w", period.Start.Format("2006-01-02"), money.ErrCurrencyMismatch)
		}
		if i == 0 {
			carry = money.Zero(period.Base.Currency)
		} else if carry.Currency != period.Base.Currency {
			return nil, fmt.Errorf("rollover period %s currency changed from %s to %s", period.Start.Format("2006-01-02"), carry.Currency, period.Base.Currency)
		}
		if i > 0 && capPeriods > 0 && carry.Amount > 0 {
			ceiling, err := period.Base.Mul(int64(capPeriods))
			if err != nil {
				return nil, fmt.Errorf("rollover period %s carry-in ceiling: %w", period.Start.Format("2006-01-02"), err)
			}
			if carry.Amount > ceiling.Amount {
				carry = ceiling
			}
		}
		effective, err := period.Base.Add(carry)
		if err != nil {
			return nil, fmt.Errorf("rollover period %s effective cap: %w", period.Start.Format("2006-01-02"), err)
		}
		carryOut, err := effective.Sub(period.Spend)
		if err != nil {
			return nil, fmt.Errorf("rollover period %s carry-out: %w", period.Start.Format("2006-01-02"), err)
		}
		out = append(out, RolloverPeriodState{Start: period.Start, Base: period.Base, CarryIn: carry, EffectiveCap: effective, Spend: period.Spend, CarryOut: carryOut})
		carry = carryOut
	}
	return out, nil
}
