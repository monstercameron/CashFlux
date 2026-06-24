// SPDX-License-Identifier: MIT

// Package runway bridges the household's recurring cash flows into the pure
// cashflow projection engine: it expands each domain.Recurring into the concrete
// dated events that fall within a horizon, then (optionally) runs the daily
// balance projection. This keeps the screen free of scheduling/FX logic — it asks
// "what's my cash runway?" and gets a deterministic answer.
//
// Amounts are integer minor units in the base currency (inflows positive, outflows
// negative, matching cashflow.Event). Pure Go, no platform dependencies; unit-
// tested on native Go.
package runway

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// dayOffset returns the whole-day count from `from`'s calendar day to `t`'s, both
// taken as calendar dates (time-of-day and location ignored), so a same-day event
// is day 0 and tomorrow is day 1 regardless of clocks or DST.
func dayOffset(from, t time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours()) / 24
}

// Events expands recurring cash flows into the dated cashflow events that fall in
// the horizon [day 0, day days) starting at `from` (day 0 = from's date). Each
// recurring item is stepped forward from its NextDue by its cadence; an item whose
// NextDue is already in the past is fast-forwarded to its first occurrence on or
// after `from`. Every in-horizon occurrence becomes one Event at its day offset,
// with the amount converted to the base currency (sign preserved, so outflows stay
// negative). A non-positive horizon yields no events.
func Events(recs []domain.Recurring, from time.Time, days int, rates currency.Rates) ([]cashflow.Event, error) {
	if days <= 0 {
		return nil, nil
	}
	var out []cashflow.Event
	for _, r := range recs {
		conv, err := rates.Convert(r.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		amt := conv.Amount
		occ := r.NextDue
		// Fast-forward a stale schedule up to the first occurrence on/after `from`.
		for dayOffset(from, occ) < 0 {
			occ = r.Cadence.Next(occ)
		}
		for {
			d := dayOffset(from, occ)
			if d >= days {
				break
			}
			out = append(out, cashflow.Event{Day: d, Amount: amt, Label: r.Label})
			occ = r.Cadence.Next(occ)
		}
	}
	return out, nil
}

// Project expands recs and runs the daily cash-flow projection from startBal over
// `days` days with the given buffer — the full "cash runway" in one call, flagging
// the first day the balance would dip below buffer.
func Project(startBal int64, recs []domain.Recurring, from time.Time, days int, buffer int64, rates currency.Rates) (cashflow.Projection, error) {
	events, err := Events(recs, from, days, rates)
	if err != nil {
		return cashflow.Projection{}, err
	}
	return cashflow.DailyBalances(startBal, events, days, buffer), nil
}
