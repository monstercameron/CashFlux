// SPDX-License-Identifier: MIT

package dashboardview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/nwchange"
)

// NetWorthTrend is the hero sparkline in minor units and the canonical
// month-to-date movement used by every other net-worth presentation.
type NetWorthTrend struct {
	Spark  []float64
	Change nwchange.Change
}

// BuildNetWorthTrend calculates the trailing six-month shape and month-to-date
// delta over the same account scope and FX table. The current spark point
// retains its instant cutoff; the delta includes all of today per nwchange.
func BuildNetWorthTrend(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, now time.Time) NetWorthTrend {
	const months = 6
	cur := dateutil.MonthStart(now)
	cutoffs := make([]time.Time, 0, months+3)
	for i := months - 1; i >= 0; i-- {
		cutoffs = append(cutoffs, dateutil.AddMonths(cur, -i))
	}
	cutoffs = append(cutoffs, now)
	window := nwchange.MonthToDate(now)
	// The sparkline already includes the start of the current month. Add only
	// the end-of-today cutoff instead of recomputing a second two-point series.
	cutoffs = append(cutoffs, window.Until)
	series, err := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	if err != nil || len(series) != len(cutoffs) {
		return NetWorthTrend{}
	}
	out := NetWorthTrend{Spark: make([]float64, months+1), Change: nwchange.Change{
		Window: window, Base: rates.Base, Known: true,
		StartMinor: series[months-1].Amount, EndMinor: series[months+1].Amount,
	}}
	for i := range out.Spark {
		out.Spark[i] = float64(series[i].Amount)
	}
	return out
}
