// SPDX-License-Identifier: MIT

package notify_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/notify"
)

func TestDefaultRuleConfig_AllEnabled(t *testing.T) {
	cfg := notify.DefaultRuleConfig()
	for _, r := range notify.DefaultRules() {
		if !cfg.IsEnabled(r.ID) {
			t.Errorf("DefaultRuleConfig: rule %q should be enabled by default", r.ID)
		}
	}
}

func TestDefaultRuleConfig_ThresholdsNotNil(t *testing.T) {
	cfg := notify.DefaultRuleConfig()
	if cfg.Thresholds == nil {
		t.Error("DefaultRuleConfig: Thresholds map should be non-nil")
	}
}

func TestRuleConfig_IsEnabled_MissingDefaultsOn(t *testing.T) {
	cfg := notify.RuleConfig{Enabled: map[string]bool{"default-bill-due": false}}
	// absent key should default to true
	if !cfg.IsEnabled("some-future-rule") {
		t.Error("IsEnabled: absent rule should default to true")
	}
}

func TestRuleConfig_IsEnabled_NilDefaultsOn(t *testing.T) {
	var cfg notify.RuleConfig
	if !cfg.IsEnabled("anything") {
		t.Error("IsEnabled: nil config should default to true")
	}
}

func TestEnabledRules_FilterDisabled(t *testing.T) {
	all := notify.DefaultRules()
	// disable the first rule
	disabledID := all[0].ID
	cfg := notify.DefaultRuleConfig()
	cfg.Enabled[disabledID] = false

	filtered := notify.EnabledRules(all, cfg)
	if len(filtered) != len(all)-1 {
		t.Errorf("EnabledRules: want %d rules, got %d", len(all)-1, len(filtered))
	}
	for _, r := range filtered {
		if r.ID == disabledID {
			t.Errorf("EnabledRules: disabled rule %q should not appear in output", disabledID)
		}
	}
}

func TestEnabledRules_AllEnabled(t *testing.T) {
	all := notify.DefaultRules()
	cfg := notify.DefaultRuleConfig()
	filtered := notify.EnabledRules(all, cfg)
	if len(filtered) != len(all) {
		t.Errorf("EnabledRules: all enabled — want %d, got %d", len(all), len(filtered))
	}
}

func TestEnabledRules_EmptyConfig(t *testing.T) {
	// empty config (not nil) — absent keys default to enabled
	all := notify.DefaultRules()
	cfg := notify.RuleConfig{}
	filtered := notify.EnabledRules(all, cfg)
	if len(filtered) != len(all) {
		t.Errorf("EnabledRules: empty config — want %d, got %d", len(all), len(filtered))
	}
}

func TestMarshalUnmarshalRuleConfig_RoundTrip(t *testing.T) {
	orig := notify.DefaultRuleConfig()
	orig.Enabled["default-stale"] = false

	raw := notify.MarshalRuleConfig(orig)
	if raw == "" {
		t.Fatal("MarshalRuleConfig returned empty string")
	}
	got := notify.UnmarshalRuleConfig(raw)
	for id, want := range orig.Enabled {
		if got.IsEnabled(id) != want {
			t.Errorf("round-trip: rule %q: want enabled=%v, got enabled=%v", id, want, got.IsEnabled(id))
		}
	}
}

func TestUnmarshalRuleConfig_EmptyString_ReturnsDefault(t *testing.T) {
	cfg := notify.UnmarshalRuleConfig("")
	for _, r := range notify.DefaultRules() {
		if !cfg.IsEnabled(r.ID) {
			t.Errorf("UnmarshalRuleConfig empty: rule %q should be enabled", r.ID)
		}
	}
}

func TestUnmarshalRuleConfig_Garbage_ReturnsDefault(t *testing.T) {
	cfg := notify.UnmarshalRuleConfig("{not valid json")
	for _, r := range notify.DefaultRules() {
		if !cfg.IsEnabled(r.ID) {
			t.Errorf("UnmarshalRuleConfig garbage: rule %q should be enabled", r.ID)
		}
	}
}

// TestUnmarshalRuleConfig_LegacyBoolMap verifies that a legacy bare map[string]bool
// payload (written before the struct migration) is promoted correctly.
func TestUnmarshalRuleConfig_LegacyBoolMap(t *testing.T) {
	legacy := `{"default-bill-due":false,"default-stale":true}`
	cfg := notify.UnmarshalRuleConfig(legacy)
	if cfg.IsEnabled("default-bill-due") {
		t.Error("legacy promotion: default-bill-due should be disabled")
	}
	if !cfg.IsEnabled("default-stale") {
		t.Error("legacy promotion: default-stale should be enabled")
	}
	if cfg.Thresholds == nil {
		t.Error("legacy promotion: Thresholds should be non-nil")
	}
}

// TestEffectiveThreshold covers the five key cases.
func TestEffectiveThreshold(t *testing.T) {
	tests := []struct {
		name        string
		ruleID      string
		cfg         notify.RuleConfig
		ruleDefault int64
		want        int64
	}{
		{
			name:   "override present and positive returns override",
			ruleID: "default-large",
			cfg: notify.RuleConfig{
				Thresholds: map[string]int64{"default-large": 100000},
			},
			ruleDefault: 50000,
			want:        100000,
		},
		{
			name:        "rule absent from map returns default",
			ruleID:      "default-large",
			cfg:         notify.RuleConfig{Thresholds: map[string]int64{}},
			ruleDefault: 50000,
			want:        50000,
		},
		{
			name:   "override is zero returns default",
			ruleID: "default-large",
			cfg: notify.RuleConfig{
				Thresholds: map[string]int64{"default-large": 0},
			},
			ruleDefault: 50000,
			want:        50000,
		},
		{
			name:   "override is negative returns default",
			ruleID: "default-large",
			cfg: notify.RuleConfig{
				Thresholds: map[string]int64{"default-large": -1},
			},
			ruleDefault: 50000,
			want:        50000,
		},
		{
			name:        "Thresholds map is nil returns default",
			ruleID:      "default-large",
			cfg:         notify.RuleConfig{},
			ruleDefault: 50000,
			want:        50000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notify.EffectiveThreshold(tt.ruleID, tt.cfg, tt.ruleDefault)
			if got != tt.want {
				t.Errorf("EffectiveThreshold(%q, cfg, %d) = %d, want %d", tt.ruleID, tt.ruleDefault, got, tt.want)
			}
		})
	}
}
