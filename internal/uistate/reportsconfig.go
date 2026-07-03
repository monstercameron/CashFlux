// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const reportsConfigKey = "cashflux:reports:config"

// reportsViews are the valid report-type tabs, used to sanitize a persisted view.
var reportsViews = map[string]bool{"overview": true, "categories": true, "networth": true, "advanced": true}

// ReportsConfig is the persisted reports-page reading posture: which report
// tab the user was on and how they compare (year-over-year vs previous period,
// sub-categories rolled up or expanded). Persisting it means /reports reopens
// exactly how it was being read — the toggles were previously transient and
// reset on every visit.
type ReportsConfig struct {
	// View is the active report tab: overview | categories | networth | advanced.
	View string `json:"view"`
	// YoY compares against the same window one year prior instead of the
	// immediately preceding window.
	YoY bool `json:"yoy"`
	// Rollup folds sub-categories into their top-level parent in the
	// by-category breakdown.
	Rollup bool `json:"rollup"`
}

func defaultReportsConfig() ReportsConfig {
	return ReportsConfig{View: "overview"}
}

// ReportsConfigGet returns the effective reports config: the persisted override
// layered over the defaults, or the plain defaults when nothing is stored.
func ReportsConfigGet() ReportsConfig {
	cfg := defaultReportsConfig()
	raw := kvGet(reportsConfigKey)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultReportsConfig()
	}
	if !reportsViews[cfg.View] {
		cfg.View = "overview"
	}
	return cfg
}

// SetReportsConfig persists a reports config override.
func SetReportsConfig(cfg ReportsConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(reportsConfigKey, string(data))
	}
}
