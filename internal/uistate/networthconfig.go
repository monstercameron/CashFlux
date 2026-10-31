// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const netWorthConfigKey = "cashflux:networth:config"

// NetWorthConfigGet returns the effective net-worth config: the persisted
// override layered over the defaults, or the plain defaults when nothing is
// stored.
func NetWorthConfigGet() NetWorthConfig {
	c, err := ParseNetWorthConfig(kvGet(netWorthConfigKey))
	if err != nil {
		return defaultNetWorthConfig()
	}
	return c
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
