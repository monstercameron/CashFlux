// SPDX-License-Identifier: MIT

package notify

import "encoding/json"

// RuleConfig is a per-ruleID enabled flag, persisted as a JSON object so new
// rules added to DefaultRules() are automatically treated as enabled until the
// user explicitly disables them.
type RuleConfig map[string]bool

// DefaultRuleConfig returns a config with every default rule enabled.
func DefaultRuleConfig() RuleConfig {
	cfg := make(RuleConfig, len(DefaultRules()))
	for _, r := range DefaultRules() {
		cfg[r.ID] = true
	}
	return cfg
}

// IsEnabled reports whether ruleID is enabled in the config. Rules absent from
// the map (e.g. newly added rules not yet in a persisted config) are treated as
// enabled by default.
func (c RuleConfig) IsEnabled(ruleID string) bool {
	if c == nil {
		return true
	}
	v, ok := c[ruleID]
	if !ok {
		return true // new rules default on
	}
	return v
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
// JSON-encoded RuleConfig (map[string]bool). It is a package-level constant so
// both the persistence helpers and callers can reference it without coupling to
// the uistate package (which has js build constraints).
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
// gracefully.
func UnmarshalRuleConfig(raw string) RuleConfig {
	if raw == "" {
		return DefaultRuleConfig()
	}
	cfg := make(RuleConfig)
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultRuleConfig()
	}
	return cfg
}
