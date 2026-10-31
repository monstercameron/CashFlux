// SPDX-License-Identifier: MIT

package dashboardview

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

var errEmptyTrend = errors.New("dashboard trend has no points")

// TrendFacts contains checked movement and range values for the dashboard's
// trend chart. Delta and range availability are independent: an overflowing
// endpoint difference does not make the plotted points or range unknown.
type TrendFacts struct {
	Delta          money.Money
	DeltaMagnitude money.Money
	DeltaUp        bool
	DeltaAvailable bool
	DeltaError     error
	Low            money.Money
	High           money.Money
	RangeAvailable bool
	RangeError     error
}

// PrepareTrendFacts derives checked endpoint movement and the chart's displayed
// range, including the live net-worth point when it lies outside the series.
func PrepareTrendFacts(series []money.Money, live money.Money) TrendFacts {
	if len(series) == 0 {
		return TrendFacts{DeltaError: errEmptyTrend, RangeError: errEmptyTrend}
	}

	out := TrendFacts{Low: series[0], High: series[0]}
	for i, point := range series {
		if point.Currency != live.Currency {
			err := fmt.Errorf("dashboard trend point %d currency %q does not match live currency %q: %w", i, point.Currency, live.Currency, money.ErrCurrencyMismatch)
			return TrendFacts{DeltaError: err, RangeError: err}
		}
		if point.Amount < out.Low.Amount {
			out.Low = point
		}
		if point.Amount > out.High.Amount {
			out.High = point
		}
	}

	first, last := series[0], series[len(series)-1]
	delta, err := last.Sub(first)
	out.Delta, out.DeltaError = delta, err
	if err == nil {
		out.DeltaUp = delta.Amount > 0
		magnitude := delta
		if delta.Amount < 0 {
			magnitude, err = delta.AbsChecked()
		}
		if err == nil {
			out.DeltaMagnitude = magnitude
			out.DeltaAvailable = true
		} else {
			out.DeltaError = err
		}
	}

	if live.Amount < out.Low.Amount {
		out.Low = live
	}
	if live.Amount > out.High.Amount {
		out.High = live
	}
	out.RangeAvailable = true
	return out
}
