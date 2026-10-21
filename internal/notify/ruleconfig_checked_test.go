// SPDX-License-Identifier: MIT

package notify

import (
	"reflect"
	"testing"
)

func TestParseRuleConfigPreservesCurrentAndLegacyInterpretation(t *testing.T) {
	for _, raw := range []string{"", "{}", `{"default-budget":false,"new-rule":true}`, `{"enabled":{"default-budget":false},"thresholds":{"default-bill-due":10},"members":{"default-budget":"member"},"quietStartMin":1320,"quietEndMin":420,"digestCadence":"monthly"}`, `{"thresholds":{"default-bill-due":10}}`} {
		cfg, err := ParseRuleConfig(raw)
		if err != nil || !reflect.DeepEqual(cfg, UnmarshalRuleConfig(raw)) {
			t.Fatal(raw, cfg, err)
		}
	}
	legacy, err := ParseRuleConfig(`{"default-budget":false}`)
	if err != nil || legacy.IsEnabled("default-budget") || !legacy.IsEnabled("new-rule") || legacy.Thresholds == nil {
		t.Fatal(legacy, err)
	}
	current, err := ParseRuleConfig(`{"enabled":{"default-budget":false},"thresholds":{"default-bill-due":10},"quietStartMin":1320,"quietEndMin":420,"digestCadence":"monthly"}`)
	if err != nil || current.IsEnabled("default-budget") || EffectiveThreshold("default-bill-due", current, 14) != 10 || current.EffectiveDigestCadence() != DigestMonthly || !current.QuietHoursEnabled() {
		t.Fatal(current, err)
	}
}

func TestParseRuleConfigRejectsBadStorageWhileLegacyGetterFallsBack(t *testing.T) {
	for _, raw := range []string{"{bad", "null", "[]", "true", " ", `{"enabled":false}`, `{"enabled":{"x":"false"}}`} {
		if _, err := ParseRuleConfig(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
		if cfg := UnmarshalRuleConfig(raw); !reflect.DeepEqual(cfg, DefaultRuleConfig()) {
			t.Errorf("legacy fallback changed for %q: %+v", raw, cfg)
		}
	}
}
