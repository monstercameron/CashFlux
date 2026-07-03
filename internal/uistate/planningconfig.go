// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const planningConfigKey = "cashflux:planning:config"

// PlanningConfig is the persisted planning-page policy: the tunables the tiles used to keep in
// transient state (the cash-runway buffer + horizon, the forecast horizon, the affordability
// reserve). Persisting it means the settings survive a reload AND become engine data — the
// runway_buffer / runway_days / forecast_horizon variables the FormulaBuilder and dashboard
// widgets can read (see engineenv.addPlanningVars). Amounts are minor units of the base currency.
type PlanningConfig struct {
	// RunwayBufferMinor is the liquidity floor the cash-runway warns when the balance dips below.
	RunwayBufferMinor int64 `json:"runwayBufferMinor"`
	// RunwayDays is the cash-runway projection horizon in days (default 60).
	RunwayDays int `json:"runwayDays"`
	// ForecastMonths is the net-worth forecast horizon in months (default 12).
	ForecastMonths int `json:"forecastMonths"`
	// AffordReserveMinor seeds the "can I afford it?" buffer input.
	AffordReserveMinor int64 `json:"affordReserveMinor"`
}

func defaultPlanningConfig() PlanningConfig {
	return PlanningConfig{RunwayDays: 60, ForecastMonths: 12}
}

// PlanningConfigGet returns the effective planning config: the persisted override layered over
// the defaults, or the plain defaults when nothing is stored.
func PlanningConfigGet() PlanningConfig {
	cfg := defaultPlanningConfig()
	raw := kvGet(planningConfigKey)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultPlanningConfig()
	}
	if cfg.RunwayDays <= 0 {
		cfg.RunwayDays = 60
	}
	if cfg.ForecastMonths <= 0 {
		cfg.ForecastMonths = 12
	}
	if cfg.RunwayBufferMinor < 0 {
		cfg.RunwayBufferMinor = 0
	}
	if cfg.AffordReserveMinor < 0 {
		cfg.AffordReserveMinor = 0
	}
	return cfg
}

// SetPlanningConfig persists a planning config override.
func SetPlanningConfig(cfg PlanningConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(planningConfigKey, string(data))
	}
}
