// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// allocConfigKey is the app-KV key holding the allocate-page planning inputs.
const allocConfigKey = "cashflux:allocate:config"

// AllocConfigGet returns the effective allocate config: the persisted plan layered over the
// defaults, or the plain defaults when nothing is stored.
func AllocConfigGet() AllocConfig {
	cfg, err := ParseAllocConfig(kvGet(allocConfigKey))
	if err != nil {
		return defaultAllocConfig()
	}
	return cfg
}

// SetAllocConfig persists the allocate plan.
func SetAllocConfig(cfg AllocConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(allocConfigKey, string(data))
	}
}
