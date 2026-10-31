// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const planningConfigKey = "cashflux:planning:config"

// PlanningConfigGet returns the effective planning config: the persisted override layered over
// the defaults, or the plain defaults when nothing is stored.
func PlanningConfigGet() PlanningConfig {
	cfg, err := ParsePlanningConfig(kvGet(planningConfigKey))
	if err != nil {
		return defaultPlanningConfig()
	}
	return cfg
}

// SetPlanningConfig persists a planning config override.
func SetPlanningConfig(cfg PlanningConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(planningConfigKey, string(data))
	}
}
