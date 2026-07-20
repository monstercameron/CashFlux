// SPDX-License-Identifier: MIT

package notify_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/notify"
)

// TestRuleConfig_QuietHours_RoundTrip verifies quiet-hours + digest-cadence
// fields survive a marshal/unmarshal round-trip (C416).
func TestRuleConfig_QuietHours_RoundTrip(t *testing.T) {
	orig := notify.DefaultRuleConfig()
	orig.QuietStartMin = 22 * 60 // 22:00
	orig.QuietEndMin = 7 * 60    // 07:00
	orig.DigestCadence = notify.DigestMonthly

	raw := notify.MarshalRuleConfig(orig)
	if raw == "" {
		t.Fatal("MarshalRuleConfig returned empty string")
	}
	got := notify.UnmarshalRuleConfig(raw)
	if got.QuietStartMin != orig.QuietStartMin || got.QuietEndMin != orig.QuietEndMin {
		t.Errorf("quiet hours round-trip: got %d..%d, want %d..%d",
			got.QuietStartMin, got.QuietEndMin, orig.QuietStartMin, orig.QuietEndMin)
	}
	if got.EffectiveDigestCadence() != notify.DigestMonthly {
		t.Errorf("digest cadence round-trip: got %q, want monthly", got.EffectiveDigestCadence())
	}
}

// TestRuleConfig_QuietHoursEnabled covers the zero-width (off) case.
func TestRuleConfig_QuietHoursEnabled(t *testing.T) {
	off := notify.RuleConfig{}
	if off.QuietHoursEnabled() {
		t.Error("empty config: quiet hours should be off")
	}
	on := notify.RuleConfig{QuietStartMin: 1320, QuietEndMin: 420}
	if !on.QuietHoursEnabled() {
		t.Error("22:00–07:00: quiet hours should be on")
	}
	zeroWidth := notify.RuleConfig{QuietStartMin: 600, QuietEndMin: 600}
	if zeroWidth.QuietHoursEnabled() {
		t.Error("equal start/end: quiet hours should be off")
	}
}

// TestRuleConfig_InQuietHours checks the wrap-past-midnight window against the
// per-rule math (they must agree — the config delegates to Rule.InQuietHours).
func TestRuleConfig_InQuietHours(t *testing.T) {
	cfg := notify.RuleConfig{QuietStartMin: 22 * 60, QuietEndMin: 7 * 60} // 22:00–07:00
	at := func(h, m int) time.Time { return time.Date(2026, 7, 19, h, m, 0, 0, time.Local) }
	cases := []struct {
		h, m int
		want bool
	}{
		{23, 30, true},  // inside, before midnight
		{2, 0, true},    // inside, after midnight
		{6, 59, true},   // last quiet minute
		{7, 0, false},   // end is exclusive
		{12, 0, false},  // midday
		{21, 59, false}, // one minute before quiet
		{22, 0, true},   // start is inclusive
	}
	for _, c := range cases {
		if got := cfg.InQuietHours(at(c.h, c.m)); got != c.want {
			t.Errorf("InQuietHours(%02d:%02d) = %v, want %v", c.h, c.m, got, c.want)
		}
	}
}

// TestRuleConfig_EffectiveDigestCadence covers the default + explicit cases.
func TestRuleConfig_EffectiveDigestCadence(t *testing.T) {
	if got := (notify.RuleConfig{}).EffectiveDigestCadence(); got != notify.DigestWeekly {
		t.Errorf("unset cadence: got %q, want weekly", got)
	}
	if got := (notify.RuleConfig{DigestCadence: notify.DigestMonthly}).EffectiveDigestCadence(); got != notify.DigestMonthly {
		t.Errorf("monthly cadence: got %q, want monthly", got)
	}
	if got := (notify.RuleConfig{DigestCadence: "garbage"}).EffectiveDigestCadence(); got != notify.DigestWeekly {
		t.Errorf("unrecognized cadence: got %q, want weekly fallback", got)
	}
}

// TestUnmarshalRuleConfig_LegacyNoQuietFields verifies a payload written before
// C416 (no quiet/digest keys) unmarshals with quiet hours off and weekly digest.
func TestUnmarshalRuleConfig_LegacyNoQuietFields(t *testing.T) {
	legacy := `{"enabled":{"default-stale":false},"thresholds":{}}`
	cfg := notify.UnmarshalRuleConfig(legacy)
	if cfg.QuietHoursEnabled() {
		t.Error("legacy payload: quiet hours should default off")
	}
	if cfg.EffectiveDigestCadence() != notify.DigestWeekly {
		t.Errorf("legacy payload: digest cadence should default weekly, got %q", cfg.EffectiveDigestCadence())
	}
}
