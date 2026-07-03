// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAddPlanningVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	// A plan: start $1,000, −$100/mo over 12 months → depletes at ~10 months, end −$200.
	plan := domain.Plan{
		ID: "p1", Name: "Sabbatical", HorizonMonths: 12, StartBalance: 100000,
		Items: []domain.PlanItem{{ID: "i1", Kind: domain.PlanItemRecurring, Amount: -10000}},
	}
	pd := PlanningData{RunwayBufferMinor: 50000, RunwayDays: 90, ForecastMonths: 18}
	vars := Vars(Data{Plans: []domain.Plan{plan}, Planning: pd, Rates: currency.Rates{Base: "USD"}, Now: now})

	if got := vars["runway_buffer"]; got != 500 {
		t.Errorf("runway_buffer = %v, want 500", got)
	}
	if got := vars["runway_days"]; got != 90 {
		t.Errorf("runway_days = %v, want 90", got)
	}
	if got := vars["forecast_horizon"]; got != 18 {
		t.Errorf("forecast_horizon = %v, want 18", got)
	}
	if got := vars["plan_sabbatical_monthly"]; got != -100 {
		t.Errorf("plan_sabbatical_monthly = %v, want -100", got)
	}
	if got := vars["plan_sabbatical_end"]; got != -200 {
		t.Errorf("plan_sabbatical_end = %v, want -200", got)
	}
	// It depletes within the horizon, so runway months is a positive number.
	if got := vars["plan_sabbatical_runway"]; got <= 0 || got > 12 {
		t.Errorf("plan_sabbatical_runway = %v, want (0,12]", got)
	}
}

func TestAddPlanningVarsEmpty(t *testing.T) {
	vars := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	// Fixed policy vars are always present; defaults are zero when no config is fed in.
	for _, k := range []string{"runway_buffer", "runway_days", "forecast_horizon"} {
		if _, ok := vars[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
}
