// SPDX-License-Identifier: MIT

package smart

import (
	"reflect"
	"testing"
)

// freeCodes returns the catalog codes for all Free-tier features, for use in
// migration test assertions.
func freeCodes() []string {
	var out []string
	for _, f := range catalog {
		if f.Tier == TierFree {
			out = append(out, f.Code)
		}
	}
	return out
}

// TestMigrateExplicitOffPreserved verifies that a legacy row with a Free
// feature in ExplicitOff keeps that explicit-off after migration — user
// choices are never overridden.
func TestMigrateExplicitOffPreserved(t *testing.T) {
	free := freeCodes()
	if len(free) == 0 {
		t.Skip("no Free features in catalog")
	}
	target := free[0]

	// Legacy row: Version==0, one Free feature explicitly turned off.
	legacy := Settings{}.SetEnabled(target, false) // lands in ExplicitOff

	got := Migrate(legacy)

	if got.IsEnabled(target) {
		t.Errorf("Migrate: Free feature %q was in ExplicitOff but became enabled after migration", target)
	}
	if !got.ExplicitOff[target] {
		t.Errorf("Migrate: ExplicitOff entry for %q was cleared by migration", target)
	}
	if got.Version != CurrentSettingsVersion {
		t.Errorf("Migrate: Version = %d, want %d", got.Version, CurrentSettingsVersion)
	}
}

// TestMigrateUnsetFreeBecomesOn verifies that a Free feature with no
// explicit state in a legacy row gets the free-on default applied by Migrate.
func TestMigrateUnsetFreeBecomesOn(t *testing.T) {
	free := freeCodes()
	if len(free) == 0 {
		t.Skip("no Free features in catalog")
	}

	// Legacy zero row: no explicit choices at all.
	got := Migrate(Settings{})

	for _, code := range free {
		if !got.IsEnabled(code) {
			t.Errorf("Migrate: unset Free feature %q should be on after migration, but IsEnabled = false", code)
		}
		if !got.Enabled[code] {
			t.Errorf("Migrate: unset Free feature %q should be in Enabled map after migration", code)
		}
	}
	if got.Version != CurrentSettingsVersion {
		t.Errorf("Migrate: Version = %d, want %d", got.Version, CurrentSettingsVersion)
	}
}

// TestMigrateAlreadyMigratedRowUnchanged verifies that a row with Version
// already set to CurrentSettingsVersion is returned without modification.
func TestMigrateAlreadyMigratedRowUnchanged(t *testing.T) {
	s := Settings{Version: CurrentSettingsVersion}.
		SetEnabled("SMART-A1", false) // explicit-off on a Free feature

	got := Migrate(s)

	if !reflect.DeepEqual(s, got) {
		t.Errorf("Migrate: already-migrated row was modified\n  before: %+v\n   after: %+v", s, got)
	}
	// Specifically: the explicit-off must survive untouched.
	if got.IsEnabled("SMART-A1") {
		t.Errorf("Migrate: explicit-off Free feature SMART-A1 became enabled on already-migrated row")
	}
}

// TestMigratePreservesExplicitOnAI verifies that an AI feature the user had
// explicitly enabled is not disturbed by migration.
func TestMigratePreservesExplicitOnAI(t *testing.T) {
	// Find an AI feature.
	var aiCode string
	for _, f := range catalog {
		if f.Tier == TierAI {
			aiCode = f.Code
			break
		}
	}
	if aiCode == "" {
		t.Skip("no AI features in catalog")
	}

	legacy := Settings{}.SetEnabled(aiCode, true) // explicit-on for an AI feature
	got := Migrate(legacy)

	if !got.IsEnabled(aiCode) {
		t.Errorf("Migrate: explicitly-on AI feature %q became disabled after migration", aiCode)
	}
	if got.Version != CurrentSettingsVersion {
		t.Errorf("Migrate: Version = %d, want %d", got.Version, CurrentSettingsVersion)
	}
}

// TestMigrateAIFeaturesNotEnabledByDefault verifies that AI features with no
// explicit state remain off (tier default) after migration — migration must not
// enable AI features.
func TestMigrateAIFeaturesNotEnabledByDefault(t *testing.T) {
	// A zero-value legacy row: no explicit choices.
	got := Migrate(Settings{})

	for _, f := range catalog {
		if f.Tier != TierAI {
			continue
		}
		// Should still be off (tier default); only Free features get filled in.
		if got.Enabled[f.Code] {
			t.Errorf("Migrate: AI feature %q was unexpectedly added to Enabled by migration", f.Code)
		}
	}
}

// TestMigrateMixedLegacyRow exercises a realistic mixed scenario: some Free
// features explicitly on, one Free feature explicitly off, one AI feature on,
// and several Free features with no explicit state.
func TestMigrateMixedLegacyRow(t *testing.T) {
	free := freeCodes()
	if len(free) < 2 {
		t.Skip("need at least 2 Free features")
	}
	var aiCode string
	for _, f := range catalog {
		if f.Tier == TierAI {
			aiCode = f.Code
			break
		}
	}

	explicitly_on := free[0]
	explicitly_off := free[1]

	s := Settings{}
	s = s.SetEnabled(explicitly_on, true)
	s = s.SetEnabled(explicitly_off, false) // explicit-off
	if aiCode != "" {
		s = s.SetEnabled(aiCode, true)
	}

	got := Migrate(s)

	// Explicitly-on Free feature must stay on.
	if !got.IsEnabled(explicitly_on) {
		t.Errorf("Migrate: explicitly-on Free feature %q became disabled", explicitly_on)
	}
	// Explicitly-off Free feature must stay off.
	if got.IsEnabled(explicitly_off) {
		t.Errorf("Migrate: explicitly-off Free feature %q became enabled", explicitly_off)
	}
	// AI feature must stay on.
	if aiCode != "" && !got.IsEnabled(aiCode) {
		t.Errorf("Migrate: explicitly-on AI feature %q became disabled", aiCode)
	}
	// All other Free features (no explicit state) should now be explicitly on.
	for _, f := range catalog {
		if f.Tier != TierFree || f.Code == explicitly_on || f.Code == explicitly_off {
			continue
		}
		if !got.Enabled[f.Code] {
			t.Errorf("Migrate: unset Free feature %q not added to Enabled by migration", f.Code)
		}
	}
	if got.Version != CurrentSettingsVersion {
		t.Errorf("Migrate: Version = %d, want %d", got.Version, CurrentSettingsVersion)
	}
}

// TestMigrateIdempotent verifies that calling Migrate twice on a Version-0 row
// is harmless: the second call is a no-op because Version is now set.
func TestMigrateIdempotent(t *testing.T) {
	first := Migrate(Settings{})
	second := Migrate(first)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Migrate is not idempotent: first=%+v second=%+v", first, second)
	}
}
