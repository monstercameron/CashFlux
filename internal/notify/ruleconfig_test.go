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

func TestRuleConfig_IsEnabled_MissingDefaultsOn(t *testing.T) {
	cfg := notify.RuleConfig{"default-bill-due": false}
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
	cfg[disabledID] = false

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
	orig["default-stale"] = false

	raw := notify.MarshalRuleConfig(orig)
	if raw == "" {
		t.Fatal("MarshalRuleConfig returned empty string")
	}
	got := notify.UnmarshalRuleConfig(raw)
	for id, want := range orig {
		if got[id] != want {
			t.Errorf("round-trip: rule %q: want %v, got %v", id, want, got[id])
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
