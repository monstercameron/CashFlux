// SPDX-License-Identifier: MIT

// Package planning projects a saved Plan (a named what-if scenario) into a
// balance curve by composing the pure domain Plan/PlanItem types with the
// forecast engine. It is the thin logic layer between the data model and the
// Planning UI: the UI builds and persists Plans, this package turns one into
// numbers, and internal/forecast does the month-by-month arithmetic.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package planning

import (
	"math"

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
//
// A plan with no growth and no return delegates to the forecast engine exactly
// as it always did — the linear case is not reimplemented here, so it cannot
// drift from every other forecast in the app (FP-T3a).
func Project(p domain.Plan) []int64 {
	if !isCompounding(p) {
		rec, one := toForecastInputs(p)
		return forecast.Project(p.StartBalance, rec, one, p.HorizonMonths)
	}
	return projectCompounding(p)
}

// isCompounding reports whether a plan needs the month-by-month path.
func isCompounding(p domain.Plan) bool {
	if p.AnnualReturnPct != 0 {
		return true
	}
	for _, it := range p.Items {
		if it.Kind == domain.PlanItemRecurring && it.AnnualGrowthPct != 0 {
			return true
		}
	}
	return false
}

// projectCompounding walks the horizon a month at a time, applying a return to
// the balance and letting recurring items grow on their anniversaries.
//
// ORDER MATTERS and is stated here because it is the kind of thing that is
// invisible in the output and wrong forever: the return is applied to the
// balance the month OPENED with, then the month's flows land. Crediting a return
// on money that arrives during the month would pay a full month's return on a
// deposit made on the last day.
//
// The return is applied only to a POSITIVE balance. A positive rate on a
// negative balance would make debt shrink by itself — a household in overdraft
// pays interest, it does not earn it — and a plan that quietly repaid its own
// debt would be the most flattering possible bug.
func projectCompounding(p domain.Plan) []int64 {
	if p.HorizonMonths <= 0 {
		return nil
	}
	monthlyReturn := p.AnnualReturnPct / 100.0 / 12.0
	out := make([]int64, 0, p.HorizonMonths)
	balance := p.StartBalance

	for m := 1; m <= p.HorizonMonths; m++ {
		if balance > 0 && monthlyReturn != 0 {
			balance += int64(math.Round(float64(balance) * monthlyReturn))
		}
		// Anniversaries are 0-based year counts: months 1–12 are year 0 and carry
		// no growth, month 13 opens year 1 with one year's growth applied.
		years := (m - 1) / 12
		for _, it := range p.Items {
			switch it.Kind {
			case domain.PlanItemRecurring:
				balance += grownAmount(it, years)
			case domain.PlanItemOneTime:
				if it.Month == m {
					balance += it.Amount
				}
			}
		}
		out = append(out, balance)
	}
	return out
}

// grownAmount is a recurring item's amount after a whole number of years of
// growth.
//
// Growth compounds on the item's own amount, so a 3% raise in year two is 3% of
// the already-raised figure. Rounding once per year rather than accumulating a
// rounded delta keeps a long horizon from drifting cents at a time.
func grownAmount(it domain.PlanItem, years int) int64 {
	if it.AnnualGrowthPct == 0 || years <= 0 {
		return it.Amount
	}
	factor := math.Pow(1+it.AnnualGrowthPct/100.0, float64(years))
	return int64(math.Round(float64(it.Amount) * factor))
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
