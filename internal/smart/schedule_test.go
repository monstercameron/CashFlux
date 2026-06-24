// SPDX-License-Identifier: MIT

package smart

import (
	"testing"
	"time"
)

func TestCadenceValidAndLabel(t *testing.T) {
	for _, c := range AllCadences() {
		if !c.Valid() {
			t.Errorf("cadence %q should be valid", c)
		}
		if c.Label() == "" {
			t.Errorf("cadence %q has no label", c)
		}
	}
	if Cadence("nope").Valid() {
		t.Errorf("unknown cadence reported valid")
	}
}

func TestDefaultCadence(t *testing.T) {
	if DefaultCadence(TierFree) != CadenceLive {
		t.Errorf("Free default should be Live")
	}
	if DefaultCadence(TierAI) != CadenceManual {
		t.Errorf("AI default should be Manual (click-before-run)")
	}
}

func TestCadenceDue(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	var zero time.Time

	if !CadenceLive.Due(zero, now, false, false) {
		t.Errorf("Live is always due")
	}
	if CadenceManual.Due(zero, now, true, true) {
		t.Errorf("Manual is never auto-due")
	}
	if !CadenceOnOpen.Due(now.Add(-time.Hour), now, false, true) {
		t.Errorf("OnOpen due on app open")
	}
	if CadenceOnOpen.Due(now.Add(-time.Hour), now, false, false) {
		t.Errorf("OnOpen not due mid-session")
	}
	if !CadenceOnChange.Due(now.Add(-time.Hour), now, true, false) {
		t.Errorf("OnChange due when data changed")
	}
	if CadenceOnChange.Due(now.Add(-time.Hour), now, false, false) {
		t.Errorf("OnChange not due without change")
	}
	// Daily: due after 24h, not before.
	if CadenceDaily.Due(now.Add(-2*time.Hour), now, false, false) {
		t.Errorf("Daily not due after 2h")
	}
	if !CadenceDaily.Due(now.Add(-25*time.Hour), now, false, false) {
		t.Errorf("Daily due after 25h")
	}
	// Never-run on an auto cadence is due so the first result appears.
	if !CadenceDaily.Due(zero, now, false, false) {
		t.Errorf("never-run Daily should be due")
	}
}

func TestCadenceFreshFor(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	if CadenceWeekly.FreshFor(time.Time{}, now) {
		t.Errorf("never-run is not fresh")
	}
	if !CadenceWeekly.FreshFor(now.Add(-2*24*time.Hour), now) {
		t.Errorf("weekly result 2 days old is still fresh")
	}
	if CadenceWeekly.FreshFor(now.Add(-8*24*time.Hour), now) {
		t.Errorf("weekly result 8 days old is stale")
	}
	if !CadenceManual.FreshFor(now.Add(-100*24*time.Hour), now) {
		t.Errorf("manual result stays fresh until re-run")
	}
}

func TestSettingsCadence(t *testing.T) {
	var s Settings
	// Defaults by tier.
	if s.CadenceFor("SMART-A1") != CadenceLive {
		t.Errorf("Free feature default cadence should be Live")
	}
	if s.CadenceFor("SMART-A5") != CadenceManual {
		t.Errorf("AI feature default cadence should be Manual")
	}
	// Override + persistence.
	s = s.SetCadence("SMART-A5", CadenceWeekly)
	if s.CadenceFor("SMART-A5") != CadenceWeekly {
		t.Errorf("cadence override not applied")
	}
	// Setting back to default clears the override.
	s = s.SetCadence("SMART-A5", CadenceManual)
	if _, ok := s.Schedules["SMART-A5"]; ok {
		t.Errorf("setting cadence to the default should clear the override")
	}
	// Unknown code is a no-op.
	if (Settings{}).SetCadence("SMART-NOPE", CadenceDaily).Schedules["SMART-NOPE"] != "" {
		t.Errorf("set cadence on unknown feature")
	}
}

func TestSettingsMute(t *testing.T) {
	s := Settings{}.SetEnabled("SMART-A1", true)
	if s.IsMuted("SMART-A1") {
		t.Errorf("not muted by default")
	}
	s = s.SetMuted("SMART-A1", true)
	if !s.IsMuted("SMART-A1") {
		t.Errorf("mute failed")
	}
	// A muted feature is enabled but not in ActiveCodes.
	for _, c := range s.ActiveCodes() {
		if c == "SMART-A1" {
			t.Errorf("muted feature should not be active")
		}
	}
	s = s.SetMuted("SMART-A1", false)
	if s.IsMuted("SMART-A1") {
		t.Errorf("unmute failed")
	}
}

func TestSettingsRunAndResult(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	s := Settings{}.MarkRun("SMART-A5", now)
	if got := s.LastRunAt("SMART-A5"); got.Unix() != now.Unix() {
		t.Errorf("LastRunAt = %v, want %v", got, now)
	}
	if !s.LastRunAt("SMART-A1").IsZero() {
		t.Errorf("never-run feature should have zero last-run")
	}
	s = s.SetResult("SMART-A5", "You have $4,200 in checking.")
	if s.ResultFor("SMART-A5") != "You have $4,200 in checking." {
		t.Errorf("result cache failed")
	}
}

func TestActiveDropsMuted(t *testing.T) {
	s := Settings{}.SetEnabled("SMART-A1", true).SetMuted("SMART-A1", true)
	in := []Insight{{Feature: "SMART-A1", Key: "k"}}
	if got := s.Active(in); len(got) != 0 {
		t.Errorf("muted feature's insight should be filtered, got %d", len(got))
	}
}
