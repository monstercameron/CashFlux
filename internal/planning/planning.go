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
	"errors"
	"fmt"
	"math"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/money"
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
	curve, _ := projectCompounding(p, false)
	return curve
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
func projectCompounding(p domain.Plan, checked bool) ([]int64, error) {
	if p.HorizonMonths <= 0 {
		return nil, nil
	}
	monthlyReturn := p.AnnualReturnPct / 100.0 / 12.0
	out := make([]int64, 0, p.HorizonMonths)
	balance := p.StartBalance

	for m := 1; m <= p.HorizonMonths; m++ {
		if balance > 0 && monthlyReturn != 0 {
			if checked && (math.IsNaN(p.AnnualReturnPct) || math.IsInf(p.AnnualReturnPct, 0)) {
				return nil, fmt.Errorf("planning: month %d return: %w", m, ErrInvalidAssumption)
			}
			amount, err := roundedAmount(float64(balance)*monthlyReturn, checked)
			if err != nil {
				return nil, fmt.Errorf("planning: month %d return: %w", m, err)
			}
			balance, err = addAmount(balance, amount, checked)
			if err != nil {
				return nil, fmt.Errorf("planning: month %d return balance: %w", m, err)
			}
		}
		// Anniversaries are 0-based year counts: months 1–12 are year 0 and carry
		// no growth, month 13 opens year 1 with one year's growth applied.
		years := (m - 1) / 12
		for _, it := range p.Items {
			switch it.Kind {
			case domain.PlanItemRecurring:
				amount, err := grownAmountUsing(it, years, checked)
				if err != nil {
					return nil, fmt.Errorf("planning: month %d item %q growth: %w", m, it.Label, err)
				}
				balance, err = addAmount(balance, amount, checked)
				if err != nil {
					return nil, fmt.Errorf("planning: month %d item %q: %w", m, it.Label, err)
				}
			case domain.PlanItemOneTime:
				if it.Month == m {
					var err error
					balance, err = addAmount(balance, it.Amount, checked)
					if err != nil {
						return nil, fmt.Errorf("planning: month %d one-time item %q: %w", m, it.Label, err)
					}
				}
			}
		}
		out = append(out, balance)
	}
	return out, nil
}

// grownAmountUsing is a recurring item's amount after a whole number of years of
// growth.
//
// Growth compounds on the item's own amount, so a 3% raise in year two is 3% of
// the already-raised figure. Rounding once per year rather than accumulating a
// rounded delta keeps a long horizon from drifting cents at a time.
func grownAmountUsing(it domain.PlanItem, years int, checked bool) (int64, error) {
	if it.AnnualGrowthPct == 0 || years <= 0 {
		return it.Amount, nil
	}
	if checked && it.Amount == 0 {
		return 0, nil
	}
	if checked && (math.IsNaN(it.AnnualGrowthPct) || math.IsInf(it.AnnualGrowthPct, 0)) {
		return 0, ErrInvalidAssumption
	}
	factor := math.Pow(1+it.AnnualGrowthPct/100.0, float64(years))
	if checked && factor == 1 {
		return it.Amount, nil
	}
	return roundedAmount(float64(it.Amount)*factor, checked)
}

func addAmount(a, b int64, checked bool) (int64, error) {
	if checked {
		return money.Add(a, b)
	}
	return a + b, nil
}

func roundedAmount(value float64, checked bool) (int64, error) {
	rounded := math.Round(value)
	if checked && (math.IsNaN(rounded) || math.IsInf(rounded, 0) || rounded >= 0x1p63 || rounded < -0x1p63) {
		return 0, money.ErrOverflow
	}
	return int64(rounded), nil
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

// ErrInvalidAssumption identifies an unsupported item kind or a non-finite rate
// when that rate is used by a checked projection.
var ErrInvalidAssumption = errors.New("planning: invalid assumption")

// Summary contains checked derived facts for one saved base-currency plan.
// EvaluateChecked returns a zero Summary on error; zero never stands for failure.
type Summary struct {
	EndBalance   int64
	MonthlyNet   int64
	RunwayMonths float64
	Depletes     bool
}

func checkPlan(p domain.Plan, months int) error {
	if err := forecast.CheckProjectionBounds(months, len(p.Items)); err != nil {
		return err
	}
	for i, item := range p.Items {
		if item.Kind != domain.PlanItemRecurring && item.Kind != domain.PlanItemOneTime {
			return fmt.Errorf("planning: item %d (%q) kind %q: %w", i+1, item.Label, item.Kind, ErrInvalidAssumption)
		}
	}
	return nil
}

// ProjectChecked shares Project's algorithm, cadence, item ordering and rounding,
// while checking every addition and float-to-minor conversion. Work is bounded
// by forecast.CheckProjectionBounds, and an error returns no partial curve.
func ProjectChecked(p domain.Plan) ([]int64, error) {
	if err := checkPlan(p, p.HorizonMonths); err != nil {
		return nil, err
	}
	if !isCompounding(p) {
		rec, one := toForecastInputs(p)
		return forecast.ProjectChecked(p.StartBalance, rec, one, p.HorizonMonths)
	}
	return projectCompounding(p, true)
}

// MonthlyNetChecked returns the checked base monthly recurring net. Growth and
// one-time assumptions do not enter this steady initial-month figure.
func MonthlyNetChecked(p domain.Plan) (int64, error) {
	if err := checkPlan(p, 0); err != nil {
		return 0, err
	}
	rec, _ := toForecastInputs(p)
	return forecast.MonthlyNetChecked(rec)
}

// EndBalanceChecked returns the checked last balance, or the starting balance
// for a non-positive horizon. A failed projection returns zero with an error.
func EndBalanceChecked(p domain.Plan) (int64, error) {
	curve, err := ProjectChecked(p)
	if err != nil {
		return 0, err
	}
	if len(curve) == 0 {
		return p.StartBalance, nil
	}
	return curve[len(curve)-1], nil
}

// EvaluateChecked computes all saved-plan facts from a single checked curve.
// Plan amounts are already in the household base currency; this model has no
// per-item currency and therefore performs no implicit FX conversion.
func EvaluateChecked(p domain.Plan) (Summary, error) {
	if err := checkPlan(p, p.HorizonMonths); err != nil {
		return Summary{}, err
	}
	monthly, err := MonthlyNetChecked(p)
	if err != nil {
		return Summary{}, err
	}
	curve, err := ProjectChecked(p)
	if err != nil {
		return Summary{}, err
	}
	end := p.StartBalance
	if len(curve) > 0 {
		end = curve[len(curve)-1]
	}
	months, depletes := runwayFromCurve(p.StartBalance, curve)
	return Summary{EndBalance: end, MonthlyNet: monthly, RunwayMonths: months, Depletes: depletes}, nil
}
