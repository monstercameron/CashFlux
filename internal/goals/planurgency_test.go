// SPDX-License-Identifier: MIT

package goals

import "testing"

func TestClassifyPlanUrgency(t *testing.T) {
	tests := []struct {
		name                    string
		required, surplus, fair int64
		want                    PlanUrgency
	}{
		{"nothing needed", 0, 100000, 25000, UrgencyNone},
		{"negative requirement", -500, 100000, 25000, UrgencyNone},
		{"within fair share is on track", 20000, 100000, 25000, UrgencyNone},
		{"exactly fair share is on track", 25000, 100000, 25000, UrgencyNone},
		// fair = 25000: 1.5x = 37500, 3x = 75000.
		{"just over fair share watches", 30000, 100000, 25000, UrgencyWatch},
		{"just under 1.5x still watch", 37000, 100000, 25000, UrgencyWatch},
		{"at 1.5x slips", 37500, 100000, 25000, UrgencySlipping},
		{"between 1.5x and 3x slips", 60000, 100000, 25000, UrgencySlipping},
		{"at 3x is far behind", 75000, 100000, 25000, UrgencyFarBehind},
		{"over 3x but within surplus is far behind", 90000, 100000, 25000, UrgencyFarBehind},
		{"exceeds surplus is far behind", 120000, 100000, 25000, UrgencyFarBehind},
		{"no surplus is far behind", 30000, 0, 0, UrgencyFarBehind},
		{"negative surplus is far behind", 30000, -5000, 0, UrgencyFarBehind},
		{"reachable but zero fair share is far behind", 40000, 50000, 0, UrgencyFarBehind},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyPlanUrgency(tc.required, tc.surplus, tc.fair); got != tc.want {
				t.Errorf("ClassifyPlanUrgency(%d, %d, %d) = %v, want %v",
					tc.required, tc.surplus, tc.fair, got, tc.want)
			}
		})
	}
}

// TestClassifyPlanUrgency_AgreesWithHealth checks the refinement never contradicts the
// coarse AssessHealth verdict: an At-risk goal is always FarBehind, and any goal graded
// behind (Watch/Slipping/FarBehind) is one AssessHealth would flag Watch or At risk.
func TestClassifyPlanUrgency_AgreesWithHealth(t *testing.T) {
	cases := []struct {
		required, surplus int64
		n                 int
	}{
		{30000, 100000, 4},  // watch verdict, mild stretch
		{60000, 100000, 4},  // watch verdict, slipping
		{90000, 100000, 4},  // watch verdict, far behind (>=3x fair=25000)
		{120000, 100000, 4}, // at-risk verdict
	}
	for _, c := range cases {
		fair := c.surplus / int64(c.n)
		h := AssessHealth(c.required, c.surplus, c.n)
		u := ClassifyPlanUrgency(c.required, c.surplus, fair)
		if h == HealthAtRisk && u != UrgencyFarBehind {
			t.Errorf("at-risk (%d/%d/%d) graded %v, want FarBehind", c.required, c.surplus, c.n, u)
		}
		if (h == HealthWatch || h == HealthAtRisk) && u == UrgencyNone {
			t.Errorf("behind verdict %v (%d/%d/%d) graded None", h, c.required, c.surplus, c.n)
		}
	}
}

func TestPlanUrgencyRank(t *testing.T) {
	order := []PlanUrgency{UrgencyFarBehind, UrgencySlipping, UrgencyWatch, UrgencyNone}
	for i := 1; i < len(order); i++ {
		if order[i-1].Rank() >= order[i].Rank() {
			t.Errorf("rank not strictly increasing at %d: %v(%d) vs %v(%d)",
				i, order[i-1], order[i-1].Rank(), order[i], order[i].Rank())
		}
	}
}
