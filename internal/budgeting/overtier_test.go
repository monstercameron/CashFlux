// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func TestClassifyOverage(t *testing.T) {
	cases := []struct {
		name    string
		percent int
		want    OverTier
	}{
		{"well under", 40, OverNone},
		{"near but under", 99, OverNone},
		{"exactly at cap", 100, OverMild},
		{"just over", 101, OverMild},
		{"top of mild", 109, OverMild},
		{"start of moderate", 110, OverModerate},
		{"mid moderate", 118, OverModerate},
		{"top of moderate", 124, OverModerate},
		{"start of severe", 125, OverSevere},
		{"deep severe", 260, OverSevere},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyOverage(c.percent); got != c.want {
				t.Errorf("ClassifyOverage(%d) = %v, want %v", c.percent, got, c.want)
			}
		})
	}
}

func TestOverTierClass(t *testing.T) {
	cases := map[OverTier]string{
		OverNone:     "",
		OverMild:     "over-mild",
		OverModerate: "over-mod",
		OverSevere:   "over-severe",
	}
	for tier, want := range cases {
		if got := tier.Class(); got != want {
			t.Errorf("%v.Class() = %q, want %q", tier, got, want)
		}
	}
}
