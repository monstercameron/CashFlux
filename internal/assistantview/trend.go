// SPDX-License-Identifier: MIT

package assistantview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// Trend contains the six complete monthly spending buckets shown in Insights.
// Delta compares the latest bucket to their average, in base-currency minor units.
type Trend struct {
	Flows     []reports.PeriodFlow
	Values    []float64
	Labels    []string
	Average   int64
	Delta     int64
	LastMonth string
}

// SpendTrend derives chart values and its takeaway using one clock value, so
// rendering around a month boundary cannot combine different calendar windows.
func SpendTrend(txns []domain.Transaction, rates currency.Rates, now time.Time) (Trend, error) {
	const months = 6
	current := dateutil.MonthStart(now)
	bounds := make([]time.Time, months+1)
	for i := range bounds {
		bounds[i] = dateutil.AddMonths(current, i-months)
	}
	flows, err := reports.IncomeExpenseSeries(txns, bounds, rates)
	if err != nil {
		return Trend{}, err
	}
	out := Trend{Flows: flows, Values: make([]float64, len(flows)), Labels: make([]string, len(flows))}
	// Quotients and remainders preserve integer truncation without overflowing
	// when six valid monthly amounts cannot themselves be summed in an int64.
	var remainder int64
	for i, flow := range flows {
		out.Values[i], out.Labels[i] = float64(flow.Expense), bounds[i].Format("Jan")
		out.Average += flow.Expense / months
		remainder += flow.Expense % months
	}
	out.Average += remainder / months
	if len(flows) > 0 {
		out.Delta = flows[len(flows)-1].Expense - out.Average
		out.LastMonth = bounds[len(flows)-1].Format("January")
	}
	return out, nil
}
