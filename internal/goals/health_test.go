// SPDX-License-Identifier: MIT

package goals

import "testing"

func TestAssessHealth(t *testing.T) {
	cases := []struct {
		name       string
		required   int64
		surplus    int64
		nDeadlined int
		want       Health
	}{
		{"nothing required", 0, 100000, 3, HealthNone},
		{"no free cash", 50000, 0, 3, HealthNone},
		{"negative surplus", 50000, -20000, 3, HealthNone},
		// surplus 300000 over 3 goals → fair share 100000.
		{"within fair share is on track", 90000, 300000, 3, HealthOnTrack},
		{"exactly fair share is on track", 100000, 300000, 3, HealthOnTrack},
		{"above fair share but affordable is watch", 150000, 300000, 3, HealthWatch},
		{"needs almost all slack is watch", 290000, 300000, 3, HealthWatch},
		{"exceeds all slack is at risk", 300001, 300000, 3, HealthAtRisk},
		{"far exceeds slack is at risk", 900000, 300000, 3, HealthAtRisk},
		// A single goal's fair share is the whole surplus → on track up to surplus.
		{"sole goal within surplus is on track", 300000, 300000, 1, HealthOnTrack},
		{"sole goal over surplus is at risk", 300001, 300000, 1, HealthAtRisk},
		// A zero/negative goal count is treated as one (no divide-by-zero).
		{"zero goal count treated as one", 300000, 300000, 0, HealthOnTrack},
	}
	for _, c := range cases {
		if got := AssessHealth(c.required, c.surplus, c.nDeadlined); got != c.want {
			t.Fatalf("%s: AssessHealth(%d, %d, %d) = %q, want %q", c.name, c.required, c.surplus, c.nDeadlined, got, c.want)
		}
	}
}

// TestBabyFundScenarioIsNotOnTrack pins the V-sweep's exact contradiction (C352).
//
// The Goals card said "On track" for a baby fund needing $1,840/mo while the
// Smart strip, reading the same household, said only ~$462/mo was realistically
// free. Two statements about the same goal, on the same screen, that cannot both
// be true — and the badge was the one making a claim it had not checked, because
// it was derived from calendar runway alone.
func TestBabyFundScenarioIsNotOnTrack(t *testing.T) {
	const required = 184_000 // $1,840/mo to hit the deadline
	const surplus = 46_200   // $462/mo of free cash, household-wide

	if got := AssessHealth(required, surplus, 3); got != HealthAtRisk {
		t.Errorf("AssessHealth = %q, want %q — the deadline is unreachable even if EVERY "+
			"free dollar went to this one goal, which is not \"on track\" (C352)",
			got, HealthAtRisk)
	}

	// The same goal becomes reachable-but-tight once there is enough slack to
	// cover it, but not enough for it to take only a fair share.
	if got := AssessHealth(required, 200_000, 3); got != HealthWatch {
		t.Errorf("AssessHealth = %q, want %q — fundable only by starving the other "+
			"goals is a stretch, not an assurance", got, HealthWatch)
	}
	// And genuinely on track when it fits inside its own share.
	if got := AssessHealth(required, 600_000, 3); got != HealthOnTrack {
		t.Errorf("AssessHealth = %q, want %q", got, HealthOnTrack)
	}
}

// No free cash means no verdict: the card shows nothing rather than inventing
// reassurance from a far-off deadline.
func TestNoSurplusMakesNoClaim(t *testing.T) {
	if got := AssessHealth(184_000, 0, 2); got != HealthNone {
		t.Errorf("AssessHealth with no surplus = %q, want no verdict at all", got)
	}
	if got := AssessHealth(0, 500_000, 2); got != HealthNone {
		t.Errorf("AssessHealth with nothing required = %q, want no verdict", got)
	}
}
