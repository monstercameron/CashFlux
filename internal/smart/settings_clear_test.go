// SPDX-License-Identifier: MIT

package smart

import (
	"testing"
	"time"
)

// TestClearGenerated verifies a data wipe's smart cleanup removes only the
// data-derived content (dismissals, last-run stamps, cached AI messages) and
// keeps the user's preferences (enabled/explicit-off, schedules, mutes, density).
func TestClearGenerated(t *testing.T) {
	// Pick real catalog codes so the setter methods (which no-op on unknown codes)
	// actually populate the maps.
	var aiCode, freeCode string
	for _, f := range catalog {
		if aiCode == "" && f.Tier == TierAI {
			aiCode = f.Code
		}
		if freeCode == "" && f.Tier == TierFree {
			freeCode = f.Code
		}
	}
	if aiCode == "" || freeCode == "" {
		t.Fatalf("catalog missing an AI (%q) or Free (%q) feature for the test", aiCode, freeCode)
	}

	s := Settings{Version: CurrentSettingsVersion}
	s = s.SetEnabled(aiCode, true)          // a preference
	s = s.SetEnabled(freeCode, false)       // explicit-off preference
	s = s.SetMuted(aiCode, true)            // preference
	s = s.SetCadence(aiCode, CadenceManual) // preference (if non-default)
	s = s.SetDensity(DensityMinimal)        // preference
	s = s.Dismiss("anomaly:txn-123")        // derived
	s = s.MarkRun(aiCode, time.Unix(1000, 0))
	s = s.SetResult(aiCode, "You spent a lot on coffee.") // derived "message"

	got := s.ClearGenerated()

	// Derived content gone.
	if len(got.Dismissed) != 0 {
		t.Errorf("Dismissed not cleared: %v", got.Dismissed)
	}
	if len(got.LastRun) != 0 {
		t.Errorf("LastRun not cleared: %v", got.LastRun)
	}
	if len(got.Results) != 0 {
		t.Errorf("Results not cleared: %v", got.Results)
	}
	if got.ResultFor(aiCode) != "" {
		t.Errorf("cached message survived: %q", got.ResultFor(aiCode))
	}
	if got.IsDismissed("anomaly:txn-123") {
		t.Error("dismissal survived ClearGenerated")
	}

	// Preferences preserved.
	if !got.IsEnabled(aiCode) {
		t.Error("enabled preference lost")
	}
	if got.IsEnabled(freeCode) {
		t.Error("explicit-off preference lost")
	}
	if !got.IsMuted(aiCode) {
		t.Error("mute preference lost")
	}
	if got.DensityOrDefault() != DensityMinimal {
		t.Errorf("density preference lost: %v", got.DensityOrDefault())
	}

	// The original value is not mutated (value receiver; maps re-pointed, not cleared).
	if s.ResultFor(aiCode) == "" {
		t.Error("ClearGenerated mutated the original Settings")
	}
}
