// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const netWorthConfigKey = "cashflux:networth:config"

// netWorthWindows are the valid analysis windows (months), used to sanitize a
// persisted value. One month is "this month so far"; the rest are trailing
// calendar-month windows.
var netWorthWindows = map[int]bool{1: true, 6: true, 12: true, 24: true}

// NetWorthConfig is the persisted net-worth-page reading posture: how many
// calendar months the bridge decomposes and the mirrored chart spans.
// Persisting it means /networth reopens on the window the user was studying.
type NetWorthConfig struct {
	// TrendMonths is the analysis window: 1, 6, 12 or 24 months. The name is
	// kept from the trend-only era so an already-persisted value still loads.
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
	if !netWorthWindows[cfg.TrendMonths] {
		cfg.TrendMonths = 6
	}
	return cfg
}

// SetNetWorthConfig persists a net-worth config override.
func SetNetWorthConfig(cfg NetWorthConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(netWorthConfigKey, string(data))
		// kvSet writes into the dataset KV, which is flushed on a debounce; ask
		// for the durable write now so a reload cannot lose a reading posture the
		// user just chose.
		RequestPersist()
	}
}

// netWorthViewKey is the settings KV key for the Glance | Detail choice. It
// lives in the PRESERVED settings bucket (the same one as theme and language),
// so it survives a dataset wipe; SettingKVSet already flushes a durable persist.
const netWorthViewKey = "cashflux:networth-view"

// The two first-class readings of the balance sheet.
const (
	// NetWorthViewGlance is the default: the whole story in one screen.
	NetWorthViewGlance = "glance"
	// NetWorthViewDetail is the full balance sheet as a numbered document.
	NetWorthViewDetail = "detail"
)

// NetWorthViewGet returns the persisted /networth view, defaulting to Glance.
func NetWorthViewGet() string {
	if v := SettingKVGet(netWorthViewKey); v == NetWorthViewDetail {
		return NetWorthViewDetail
	}
	return NetWorthViewGlance
}

// NetWorthViewSet persists the /networth view choice (durably).
func NetWorthViewSet(v string) {
	if v != NetWorthViewDetail {
		v = NetWorthViewGlance
	}
	SettingKVSet(netWorthViewKey, v)
}
