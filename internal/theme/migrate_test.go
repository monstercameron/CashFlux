// SPDX-License-Identifier: MIT

package theme

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/prefs"
)

func TestFromPrefsDefaultsValid(t *testing.T) {
	got := FromPrefs(prefs.Default())
	if issues := got.Validate(); len(issues) != 0 {
		t.Fatalf("theme migrated from default prefs should be valid, got %+v", issues)
	}
	if got.Density != Comfortable {
		t.Errorf("Density = %q, want comfortable", got.Density)
	}
	if got.Scale != 1.0 {
		t.Errorf("Scale = %g, want 1.0", got.Scale)
	}
}

func TestFromPrefsLightVsDark(t *testing.T) {
	dark := FromPrefs(prefs.Prefs{Theme: prefs.ThemeDark})
	light := FromPrefs(prefs.Prefs{Theme: prefs.ThemeLight})
	if dark.BgBase == light.BgBase {
		t.Errorf("dark and light should differ in BgBase; both %q", dark.BgBase)
	}
	if dark.BgBase != "#0e0e0f" {
		t.Errorf("dark BgBase = %q, want the live #0e0e0f", dark.BgBase)
	}
	if light.BgBase != "#f7f6f3" {
		t.Errorf("light BgBase = %q, want the live #f7f6f3", light.BgBase)
	}
	// Both surface palettes must stay legible.
	if issues := dark.Validate(); len(issues) != 0 {
		t.Errorf("dark migration invalid: %+v", issues)
	}
	if issues := light.Validate(); len(issues) != 0 {
		t.Errorf("light migration invalid: %+v", issues)
	}
}

func TestFromPrefsSystemFallsBackToDark(t *testing.T) {
	got := FromPrefs(prefs.Prefs{Theme: prefs.ThemeSystem})
	if got.BgBase != darkBase().BgBase {
		t.Errorf("system theme should fall back to the dark palette, got BgBase %q", got.BgBase)
	}
}

func TestFromPrefsOverlaysAccentScaleDensity(t *testing.T) {
	p := prefs.Prefs{Theme: prefs.ThemeDark, Accent: "#3344ff", Scale: 150, Compact: true}
	got := FromPrefs(p)
	if got.Accent != "#3344ff" {
		t.Errorf("Accent = %q, want overlaid #3344ff", got.Accent)
	}
	if got.Scale != 1.5 {
		t.Errorf("Scale = %g, want 1.5 (from 150%%)", got.Scale)
	}
	if got.Density != Compact {
		t.Errorf("Density = %q, want compact", got.Density)
	}
}

func TestFromPrefsMinScaleStaysValid(t *testing.T) {
	// prefs allows down to 70%; the theme engine's scale floor must accept it so
	// the migration of a user at minimum zoom doesn't produce an invalid theme.
	got := FromPrefs(prefs.Prefs{Theme: prefs.ThemeDark, Scale: prefs.ScaleMin})
	if got.Scale != 0.70 {
		t.Errorf("Scale = %g, want 0.70", got.Scale)
	}
	if issues := got.Validate(); len(issues) != 0 {
		t.Errorf("theme at minimum scale should be valid, got %+v", issues)
	}
}
