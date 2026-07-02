// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// debtConfigKey is the app-KV key holding the debt-page display/policy config.
const debtConfigKey = "cashflux:debt:config"

// DebtConfig is the persisted, user-overridable policy for the debt page. It holds the
// tunables the tiles used to hardcode — the utilization bands that tint a card's rail and
// the payoff planner's defaults — so changing them is a config write, not a code change.
// The engine still computes the raw figures (owed, utilization, APR); this config only
// decides how they're banded and what the planner seeds.
type DebtConfig struct {
	// WarnUtilizationPct: a credit card at/above this utilization tints amber (default 30 —
	// the conventional "starts to weigh on a credit score" line).
	WarnUtilizationPct int `json:"warnUtilizationPct"`
	// HighUtilizationPct: at/above this tints red / reads as critical (default 75).
	HighUtilizationPct int `json:"highUtilizationPct"`
	// DefaultStrategy seeds the payoff planner: "avalanche" (highest APR first) or
	// "snowball" (smallest balance first). Default "avalanche".
	DefaultStrategy string `json:"defaultStrategy"`
	// DefaultExtraMinor is the payoff planner's seed extra monthly payment, in minor units
	// of the base currency (default 0).
	DefaultExtraMinor int64 `json:"defaultExtraMinor"`
	// ExcludeMortgage: exclude a mortgage from the payoff plan by default (default true —
	// most people don't snowball a 30-year mortgage).
	ExcludeMortgage bool `json:"excludeMortgage"`
}

// defaultDebtConfig is the seed config used until the user overrides it — the values the
// debt tiles previously baked in as literals, now data.
func defaultDebtConfig() DebtConfig {
	return DebtConfig{
		WarnUtilizationPct: 30,
		HighUtilizationPct: 75,
		DefaultStrategy:    "avalanche",
		DefaultExtraMinor:  0,
		ExcludeMortgage:    true,
	}
}

// DebtConfigGet returns the effective debt config: the persisted override layered over the
// defaults (so a partial/old stored config still gets sane values for any missing field),
// or the plain defaults when nothing is stored.
func DebtConfigGet() DebtConfig {
	cfg := defaultDebtConfig()
	raw := kvGet(debtConfigKey)
	if raw == "" {
		return cfg
	}
	// Unmarshal onto the defaults so absent keys keep their default, not a zero value.
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultDebtConfig()
	}
	// Guard against a stored config with a nonsensical band ordering.
	if cfg.HighUtilizationPct < cfg.WarnUtilizationPct {
		cfg.HighUtilizationPct = cfg.WarnUtilizationPct
	}
	if cfg.DefaultStrategy != "snowball" && cfg.DefaultStrategy != "avalanche" {
		cfg.DefaultStrategy = "avalanche"
	}
	return cfg
}

// SetDebtConfig persists a debt config override. Passing the zero value clears the
// override (the next read falls back to the defaults).
func SetDebtConfig(cfg DebtConfig) {
	if cfg == (DebtConfig{}) {
		kvSet(debtConfigKey, "")
		return
	}
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(debtConfigKey, string(data))
	}
}

// UtilizationBand classifies a utilization percent into "good" | "warn" | "high" using the
// config's thresholds, so the tiles tint a card's rail without hardcoding the cutoffs.
func (c DebtConfig) UtilizationBand(pct float64) string {
	switch {
	case pct >= float64(c.HighUtilizationPct):
		return "high"
	case pct >= float64(c.WarnUtilizationPct):
		return "warn"
	default:
		return "good"
	}
}
