// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// allocConfigKey is the app-KV key holding the allocate-page planning inputs.
const allocConfigKey = "cashflux:allocate:config"

// AllocConfig is the persisted allocate-page plan: the money the user is putting to work and
// the split controls. Persisting it (rather than keeping it in transient UI state) means the
// plan survives a reload AND becomes engine data — the alloc_* variables the FormulaBuilder
// and dashboard widgets can read (see engineenv.addAllocVars). Amounts are minor units of the
// base currency.
type AllocConfig struct {
	// AmountMinor is the total the user wants to allocate this round.
	AmountMinor int64 `json:"amountMinor"`
	// ReserveMinor is held back from the split (an emergency buffer).
	ReserveMinor int64 `json:"reserveMinor"`
	// MaxPerMinor caps any single destination (0 = uncapped).
	MaxPerMinor int64 `json:"maxPerMinor"`
	// Profile is the active ranking profile key ("balanced","returns","safety","debt",
	// "goals", or "saved:<id>").
	Profile string `json:"profile"`
	// Mode is the split strategy: "weighted" (score-proportional) or "fill" (fill goals to
	// their target first).
	Mode string `json:"mode"`
}

// defaultAllocConfig is the seed used until the user plans something: nothing to allocate, the
// balanced profile, weighted split.
func defaultAllocConfig() AllocConfig {
	return AllocConfig{Profile: "balanced", Mode: "weighted"}
}

// AllocConfigGet returns the effective allocate config: the persisted plan layered over the
// defaults, or the plain defaults when nothing is stored.
func AllocConfigGet() AllocConfig {
	cfg := defaultAllocConfig()
	raw := kvGet(allocConfigKey)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultAllocConfig()
	}
	if cfg.Profile == "" {
		cfg.Profile = "balanced"
	}
	if cfg.Mode != "weighted" && cfg.Mode != "fill" {
		cfg.Mode = "weighted"
	}
	if cfg.AmountMinor < 0 {
		cfg.AmountMinor = 0
	}
	if cfg.ReserveMinor < 0 {
		cfg.ReserveMinor = 0
	}
	if cfg.MaxPerMinor < 0 {
		cfg.MaxPerMinor = 0
	}
	return cfg
}

// SetAllocConfig persists the allocate plan.
func SetAllocConfig(cfg AllocConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(allocConfigKey, string(data))
	}
}
