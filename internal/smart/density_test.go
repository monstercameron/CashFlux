// SPDX-License-Identifier: MIT

package smart

import "testing"

func TestDensityValidLabel(t *testing.T) {
	for _, d := range AllDensities() {
		if !d.Valid() {
			t.Errorf("density %q should be valid", d)
		}
		if d.Label() == "" {
			t.Errorf("density %q has no label", d)
		}
	}
	if Density("nope").Valid() {
		t.Errorf("unknown density reported valid")
	}
}

func TestDensityShows(t *testing.T) {
	// Off shows nothing.
	for _, a := range []Affordance{AffordanceStrip, AffordanceBadge, AffordanceTooltip, AffordanceOverlay} {
		if DensityOff.Shows(a) {
			t.Errorf("Off should show no %q", a)
		}
	}
	// Minimal: quiet affordances only.
	if !DensityMinimal.Shows(AffordanceBadge) || !DensityMinimal.Shows(AffordanceStrip) || !DensityMinimal.Shows(AffordanceEmptyState) {
		t.Errorf("Minimal should show badge/strip/empty-state")
	}
	if DensityMinimal.Shows(AffordanceTooltip) || DensityMinimal.Shows(AffordanceOverlay) {
		t.Errorf("Minimal should not show tooltip/overlay")
	}
	// Standard: adds tooltip/field-assist/section-action/widget, still no overlay.
	for _, a := range []Affordance{AffordanceTooltip, AffordanceFieldAssist, AffordanceSectionAction, AffordanceWidget, AffordanceBadge} {
		if !DensityStandard.Shows(a) {
			t.Errorf("Standard should show %q", a)
		}
	}
	if DensityStandard.Shows(AffordanceOverlay) {
		t.Errorf("Standard should not show overlay")
	}
	// Everywhere: shows all.
	for _, a := range []Affordance{AffordanceStrip, AffordanceBadge, AffordanceTooltip, AffordanceFieldAssist, AffordanceSectionAction, AffordanceWidget, AffordanceOverlay} {
		if !DensityEverywhere.Shows(a) {
			t.Errorf("Everywhere should show %q", a)
		}
	}
}

func TestSettingsDensity(t *testing.T) {
	var s Settings
	if s.DensityOrDefault() != DensityStandard {
		t.Errorf("default density should be Standard")
	}
	s = s.SetDensity(DensityEverywhere)
	if s.DensityOrDefault() != DensityEverywhere {
		t.Errorf("set density failed")
	}
	// Invalid is ignored.
	s = s.SetDensity(Density("bogus"))
	if s.DensityOrDefault() != DensityEverywhere {
		t.Errorf("invalid density should be ignored")
	}
}

func TestShowsAffordance(t *testing.T) {
	// Enabled + Standard → badge shows, overlay does not.
	s := Settings{}.SetEnabled("SMART-A1", true)
	if !s.ShowsAffordance("SMART-A1", AffordanceBadge) {
		t.Errorf("enabled feature at Standard should show a badge")
	}
	if s.ShowsAffordance("SMART-A1", AffordanceOverlay) {
		t.Errorf("Standard should not show overlay")
	}
	// Muted → nothing.
	sm := s.SetMuted("SMART-A1", true)
	if sm.ShowsAffordance("SMART-A1", AffordanceBadge) {
		t.Errorf("muted feature should show no affordance")
	}
	// Disabled → nothing.
	if (Settings{}).ShowsAffordance("SMART-A1", AffordanceBadge) {
		t.Errorf("disabled feature should show no affordance")
	}
	// Density Off → nothing even if enabled.
	off := s.SetDensity(DensityOff)
	if off.ShowsAffordance("SMART-A1", AffordanceBadge) {
		t.Errorf("density Off should suppress all affordances")
	}
}

func TestEnableDisableAll(t *testing.T) {
	all := Settings{}.EnableAll()
	if all.EnabledCount() != len(catalog) {
		t.Errorf("EnableAll should enable every feature, got %d/%d", all.EnabledCount(), len(catalog))
	}
	none := all.DisableAll()
	if none.EnabledCount() != 0 {
		t.Errorf("DisableAll should clear all, got %d", none.EnabledCount())
	}
	// DisableAll keeps schedules/mutes so intent is restorable.
	withSched := Settings{}.SetEnabled("SMART-A5", true).SetCadence("SMART-A5", CadenceWeekly).DisableAll()
	if withSched.CadenceFor("SMART-A5") != CadenceWeekly {
		t.Errorf("DisableAll should keep the schedule")
	}
}
