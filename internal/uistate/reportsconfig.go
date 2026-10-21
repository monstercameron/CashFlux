// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const reportsConfigKey = "cashflux:reports:config"

// ReportsConfigGet returns the effective reports config: the persisted override
// layered over the defaults, or the plain defaults when nothing is stored.
func ReportsConfigGet() ReportsConfig {
	c, err := ParseReportsConfig(kvGet(reportsConfigKey))
	if err != nil {
		return defaultReportsConfig()
	}
	return c
}

// SetReportsConfig persists a reports config override.
func SetReportsConfig(cfg ReportsConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(reportsConfigKey, string(data))
	}
}
