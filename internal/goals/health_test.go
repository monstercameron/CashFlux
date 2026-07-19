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
