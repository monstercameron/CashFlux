// SPDX-License-Identifier: MIT

package notify

import (
	"encoding/json"
	"time"
)

// DigestCadence names how often the periodic spending digest is generated.
// It is a stable storage value persisted in RuleConfig.
type DigestCadence string

const (
	// DigestWeekly emits a once-per-ISO-week recap of the previous week (default).
	DigestWeekly DigestCadence = "weekly"
	// DigestMonthly emits a once-per-month recap of the previous calendar month.
	DigestMonthly DigestCadence = "monthly"
)

// RuleConfig holds per-ruleID user preferences: an enabled/disabled flag for
// each rule and optional threshold overrides. It is persisted as a JSON object
// so new rules added to DefaultRules() are automatically treated as enabled
// until the user explicitly disables them.
type RuleConfig struct {
	// Enabled maps ruleID → user-set on/off. Absent keys are treated as enabled.
	Enabled map[string]bool `json:"enabled"`
	// Thresholds maps ruleID → user-set threshold override in the same units the
	// rule's default uses (minor currency units for money rules; days for bill-due).
	// A zero or negative value means "use the rule default".
	Thresholds map[string]int64 `json:"thresholds"`

	// QuietStartMin and QuietEndMin define a single account-wide do-not-disturb
	// window in minutes since local midnight, in [0, 1440). End is exclusive; when
	// the two are equal quiet hours are OFF. The window may wrap past midnight
	// (start > end), e.g. 22:00–07:00 is 1320..420. During quiet hours the feed
	// still records every alert, but browser pushes are suppressed (C416). Omitted
	// from JSON when both are zero so older payloads round-trip unchanged.
	QuietStartMin int `json:"quietStartMin,omitempty"`
	QuietEndMin   int `json:"quietEndMin,omitempty"`

	// DigestCadence selects how often the spending digest is generated
	// ("weekly" or "monthly"). Empty is treated as weekly (C416). Omitted from
	// JSON when empty so older payloads round-trip unchanged.
	DigestCadence DigestCadence `json:"digestCadence,omitempty"`
}

// QuietHoursEnabled reports whether a non-empty do-not-disturb window is set.
func (c RuleConfig) QuietHoursEnabled() bool {
	return c.QuietStartMin != c.QuietEndMin
}

// InQuietHours reports whether the local clock time of t falls inside the
// account-wide do-not-disturb window. A zero-width window (start == end)
// disables quiet hours. The window is half-open [start, end) and may wrap past
// midnight. It shares Rule.InQuietHours so the config and per-rule math agree.
func (c RuleConfig) InQuietHours(t time.Time) bool {
	return Rule{QuietStartMin: c.QuietStartMin, QuietEndMin: c.QuietEndMin}.InQuietHours(t)
}

// EffectiveDigestCadence returns the configured digest cadence, defaulting to
// DigestWeekly when unset or unrecognized.
func (c RuleConfig) EffectiveDigestCadence() DigestCadence {
	if c.DigestCadence == DigestMonthly {
		return DigestMonthly
	}
	return DigestWeekly
}

// DefaultRuleConfig returns a config with every default rule enabled and no
// threshold overrides (rules use their built-in defaults).
func DefaultRuleConfig() RuleConfig {
	rules := DefaultRules()
	enabled := make(map[string]bool, len(rules))
	for _, r := range rules {
		enabled[r.ID] = true
	}
	return RuleConfig{
		Enabled:    enabled,
		Thresholds: map[string]int64{},
	}
}

// IsEnabled reports whether ruleID is enabled in the config. Rules absent from
// the map (e.g. newly added rules not yet in a persisted config) are treated as
// enabled by default.
func (c RuleConfig) IsEnabled(ruleID string) bool {
	if c.Enabled == nil {
		return true
	}
	v, ok := c.Enabled[ruleID]
	if !ok {
		return true // new rules default on
	}
	return v
}

// EffectiveThreshold returns the override threshold from cfg for ruleID if set
// and valid (positive), otherwise returns ruleDefault.
func EffectiveThreshold(ruleID string, cfg RuleConfig, ruleDefault int64) int64 {
	if cfg.Thresholds != nil {
		if v, ok := cfg.Thresholds[ruleID]; ok && v > 0 {
			return v
		}
	}
	return ruleDefault
}

// EnabledRules filters allRules to only those whose ID is enabled in config.
// Rules not present in config default to enabled.
func EnabledRules(allRules []Rule, config RuleConfig) []Rule {
	out := make([]Rule, 0, len(allRules))
	for _, r := range allRules {
		if config.IsEnabled(r.ID) {
			out = append(out, r)
		}
	}
	return out
}

// ruleConfigKV is the KV key used to persist the rule config. The value is a
// JSON-encoded RuleConfig. It is a package-level constant so both the
// persistence helpers and callers can reference it without coupling to the
// uistate package (which has js build constraints).
const ruleConfigKV = "cashflux:notify:ruleconfig"

// RuleConfigKey returns the KV store key for the rule config — exported so the
// wasm/UI layer can read/write it via uistate.KVGet / uistate.KVSet without
// importing syscall/js into this pure package.
func RuleConfigKey() string { return ruleConfigKV }

// MarshalRuleConfig serialises cfg to a JSON string for KV storage.
// It returns "" on error (treated as absent by the load path).
func MarshalRuleConfig(cfg RuleConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

// UnmarshalRuleConfig deserialises a JSON string produced by MarshalRuleConfig.
// An empty or malformed string returns DefaultRuleConfig() so the app degrades
// gracefully. Legacy payloads that were stored as a bare map[string]bool (before
// the struct migration) are detected and promoted: the bool map becomes the
// Enabled field and Thresholds starts empty.
func UnmarshalRuleConfig(raw string) RuleConfig {
	if raw == "" {
		return DefaultRuleConfig()
	}
	// Attempt to unmarshal as the current struct shape.
	var cfg RuleConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultRuleConfig()
	}
	// If Enabled is nil the payload was either empty or a legacy bare bool-map.
	// Try the legacy path: unmarshal as map[string]bool.
	if cfg.Enabled == nil {
		var legacy map[string]bool
		if err := json.Unmarshal([]byte(raw), &legacy); err == nil && len(legacy) > 0 {
			return RuleConfig{
				Enabled:    legacy,
				Thresholds: map[string]int64{},
			}
		}
		return DefaultRuleConfig()
	}
	if cfg.Thresholds == nil {
		cfg.Thresholds = map[string]int64{}
	}
	return cfg
}
