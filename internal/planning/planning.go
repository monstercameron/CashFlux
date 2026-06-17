// Package planning projects a saved Plan (a named what-if scenario) into a
// balance curve by composing the pure domain Plan/PlanItem types with the
// forecast engine. It is the thin logic layer between the data model and the
// Planning UI: the UI builds and persists Plans, this package turns one into
// numbers, and internal/forecast does the month-by-month arithmetic.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package planning

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
)

// toForecastInputs splits a plan's assumptions into the forecast engine's
// recurring (every-month) and one-time (single-month) inputs.
func toForecastInputs(p domain.Plan) ([]forecast.Recurring, []forecast.OneTime) {
	var rec []forecast.Recurring
	var one []forecast.OneTime
	for _, it := range p.Items {
		switch it.Kind {
		case domain.PlanItemRecurring:
			rec = append(rec, forecast.Recurring{Label: it.Label, Monthly: it.Amount})
		case domain.PlanItemOneTime:
			one = append(one, forecast.OneTime{Label: it.Label, Month: it.Month, Amount: it.Amount})
		}
	}
	return rec, one
}

// Project returns the projected end-of-month balance for each month of the
// plan's horizon, starting from its StartBalance and applying recurring items
// every month plus one-time items in their scheduled month. A non-positive
// horizon yields an empty slice.
func Project(p domain.Plan) []int64 {
	rec, one := toForecastInputs(p)
	return forecast.Project(p.StartBalance, rec, one, p.HorizonMonths)
}

// MonthlyNet is the steady monthly change implied by the plan's recurring items
// (one-time items are excluded). Positive is a net monthly inflow.
func MonthlyNet(p domain.Plan) int64 {
	rec, _ := toForecastInputs(p)
	return forecast.MonthlyNet(rec)
}

// EndBalance is the plan's projected balance at the end of its horizon, or the
// StartBalance when the horizon is non-positive (nothing is projected).
func EndBalance(p domain.Plan) int64 {
	curve := Project(p)
	if len(curve) == 0 {
		return p.StartBalance
	}
	return curve[len(curve)-1]
}
