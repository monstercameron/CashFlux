// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const billsSmartConfigKey = "cashflux:bills:smartconfig"

// BillsSmartConfig is the persisted smart-bill-schedule policy: whether the
// pay-ahead plan is enabled, which schedule the bills views show, the pay
// frequency the optimizer aligns to (the payday anchor date comes from
// prefs.PayCycleAnchor), and the liquidity floor the plan must respect. Stored
// as config so the schedule is data, not code — and so the alloc-style engine
// variables derived from it are reproducible.
type BillsSmartConfig struct {
	// Enabled turns the smart schedule on (computing moves + suggestions).
	Enabled bool `json:"enabled"`
	// ViewSmart shows the smart pay-on dates in the bills list/calendar instead of
	// the raw due dates (the raw date always remains visible as the deadline).
	ViewSmart bool `json:"viewSmart"`
	// PayFrequency is "weekly", "biweekly", "semimonthly", or "monthly" (default
	// biweekly — the most common cycle).
	PayFrequency string `json:"payFrequency"`
	// MinKeepMinor is the liquidity floor the pay-ahead plan must not dip below.
	MinKeepMinor int64 `json:"minKeepMinor"`
}

func defaultBillsSmartConfig() BillsSmartConfig {
	return BillsSmartConfig{PayFrequency: "biweekly"}
}

// BillsSmartConfigGet returns the effective config: the persisted override
// layered over the defaults.
func BillsSmartConfigGet() BillsSmartConfig {
	cfg := defaultBillsSmartConfig()
	raw := kvGet(billsSmartConfigKey)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultBillsSmartConfig()
	}
	switch cfg.PayFrequency {
	case "weekly", "biweekly", "semimonthly", "monthly":
	default:
		cfg.PayFrequency = "biweekly"
	}
	if cfg.MinKeepMinor < 0 {
		cfg.MinKeepMinor = 0
	}
	return cfg
}

// SetBillsSmartConfig persists the smart-schedule config.
func SetBillsSmartConfig(cfg BillsSmartConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(billsSmartConfigKey, string(data))
	}
}
