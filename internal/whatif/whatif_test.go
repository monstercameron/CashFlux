// SPDX-License-Identifier: MIT

package whatif

import "testing"

func TestDiff(t *testing.T) {
	before := map[string]float64{"net_worth": 100000, "runway_months": 6, "rent": 2400, "unchanged": 5}
	after := map[string]float64{"net_worth": 92800, "runway_months": 4, "rent": 1800, "unchanged": 5, "new_goal_eta": 18}

	got := Diff(before, after, 0)
	if len(got) != 4 {
		t.Fatalf("want 4 changes, got %d: %+v", len(got), got)
	}
	// Largest absolute delta first: net_worth moved 7200.
	if got[0].Name != "net_worth" || got[0].Delta != -7200 {
		t.Errorf("top change = %+v", got[0])
	}
	// The added variable is flagged.
	var addedSeen bool
	for _, c := range got {
		if c.Name == "new_goal_eta" {
			if !c.Added || c.After != 18 {
				t.Errorf("added change wrong: %+v", c)
			}
			addedSeen = true
		}
		if c.Name == "unchanged" {
			t.Errorf("unchanged var should not appear: %+v", c)
		}
	}
	if !addedSeen {
		t.Error("expected new_goal_eta as an added change")
	}
}

func TestDiffRemovedAndEpsilon(t *testing.T) {
	before := map[string]float64{"gone": 10, "noisy": 100.0000001}
	after := map[string]float64{"noisy": 100.0000002}
	got := Diff(before, after, 0.001)
	if len(got) != 1 || got[0].Name != "gone" || !got[0].Removed {
		t.Fatalf("epsilon should hide noise; got %+v", got)
	}
}
