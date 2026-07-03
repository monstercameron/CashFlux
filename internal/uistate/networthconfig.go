// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const netWorthConfigKey = "cashflux:networth:config"

// netWorthHorizons are the valid trend horizons (months), used to sanitize a
// persisted value.
var netWorthHorizons = map[int]bool{6: true, 12: true, 24: true}

// NetWorthConfig is the persisted net-worth-page reading posture: how many
// months the trend chart spans. Persisting it means /networth reopens on the
// horizon the user was studying.
type NetWorthConfig struct {
	// TrendMonths is the trend-chart horizon: 6, 12, or 24 months.
	TrendMonths int `json:"trendMonths"`
}

func defaultNetWorthConfig() NetWorthConfig {
	return NetWorthConfig{TrendMonths: 6}
}

// NetWorthConfigGet returns the effective net-worth config: the persisted
// override layered over the defaults, or the plain defaults when nothing is
// stored.
func NetWorthConfigGet() NetWorthConfig {
	cfg := defaultNetWorthConfig()
	raw := kvGet(netWorthConfigKey)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultNetWorthConfig()
	}
	if !netWorthHorizons[cfg.TrendMonths] {
		cfg.TrendMonths = 6
	}
	return cfg
}

// SetNetWorthConfig persists a net-worth config override.
func SetNetWorthConfig(cfg NetWorthConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(netWorthConfigKey, string(data))
	}
}
