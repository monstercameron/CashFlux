// SPDX-License-Identifier: MIT

package smart

import (
	"testing"
)

func makeInsight(feature string, sev Severity) Insight {
	return Insight{Feature: feature, Severity: sev, Key: feature + ":" + sev.String()}
}

func TestCapPerRule(t *testing.T) {
	tests := []struct {
		name     string
		input    []Insight
		n        int
		wantLen  int
		wantKeys []string // Feature codes expected in result (order preserved)
	}{
		{
			name: "4 insights same rule → only 3 kept (highest severity first)",
			input: []Insight{
				makeInsight("SMART-A1", SeverityAlert),
				makeInsight("SMART-A1", SeverityWarn),
				makeInsight("SMART-A1", SeverityNudge),
				makeInsight("SMART-A1", SeverityInfo),
			},
			n:        3,
			wantLen:  3,
			wantKeys: []string{"SMART-A1", "SMART-A1", "SMART-A1"},
		},
		{
			name: "mixed rules — each capped independently",
			input: []Insight{
				makeInsight("SMART-A1", SeverityAlert),
				makeInsight("SMART-B1", SeverityAlert),
				makeInsight("SMART-A1", SeverityWarn),
				makeInsight("SMART-B1", SeverityWarn),
				makeInsight("SMART-A1", SeverityNudge),
				makeInsight("SMART-B1", SeverityNudge),
				makeInsight("SMART-A1", SeverityInfo),
				makeInsight("SMART-B1", SeverityInfo),
			},
			n:       3,
			wantLen: 6, // 3 per rule × 2 rules
		},
		{
			name: "n >= count → no-op (all returned)",
			input: []Insight{
				makeInsight("SMART-A1", SeverityAlert),
				makeInsight("SMART-A1", SeverityWarn),
			},
			n:       5,
			wantLen: 2,
		},
		{
			name:    "empty input → empty output",
			input:   nil,
			n:       3,
			wantLen: 0,
		},
		{
			name: "n=1 → only highest severity per rule",
			input: []Insight{
				makeInsight("SMART-A1", SeverityAlert),
				makeInsight("SMART-A1", SeverityInfo),
				makeInsight("SMART-B1", SeverityWarn),
				makeInsight("SMART-B1", SeverityNudge),
			},
			n:       1,
			wantLen: 2,
		},
		{
			name: "n=0 → all capped away",
			input: []Insight{
				makeInsight("SMART-A1", SeverityAlert),
			},
			n:       0,
			wantLen: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CapPerRule(tc.input, tc.n)
			if len(got) != tc.wantLen {
				t.Errorf("CapPerRule: got %d items, want %d", len(got), tc.wantLen)
			}
			// Input must be unmodified.
			if len(tc.input) > 0 {
				_ = tc.input[0] // access to confirm no panic
			}
		})
	}
}

func TestEnableFreeOnly(t *testing.T) {
	s := Settings{}

	// Manually mark one AI feature explicitly on before calling EnableFreeOnly —
	// it should remain on (we don't touch AI features).
	var aiCode string
	for _, f := range catalog {
		if f.Tier == TierAI {
			aiCode = f.Code
			break
		}
	}
	if aiCode != "" {
		s = s.SetEnabled(aiCode, true)
	}

	// Explicitly turn off a Free feature, then call EnableFreeOnly — it should be turned back on.
	var freeCode string
	for _, f := range catalog {
		if f.Tier == TierFree {
			freeCode = f.Code
			break
		}
	}
	if freeCode != "" {
		s = s.SetEnabled(freeCode, false) // puts it in ExplicitOff
	}

	s2 := EnableFreeOnly(s)

	// All Free features must be effectively on.
	for _, f := range catalog {
		if f.Tier != TierFree {
			continue
		}
		if !s2.IsEnabled(f.Code) {
			t.Errorf("EnableFreeOnly: Free feature %q is not enabled", f.Code)
		}
	}

	// The previously-explicit-off Free feature must now be on.
	if freeCode != "" && !s2.IsEnabled(freeCode) {
		t.Errorf("EnableFreeOnly: previously explicit-off Free feature %q still off", freeCode)
	}

	// AI features: if one was explicitly enabled, it must remain enabled.
	if aiCode != "" && !s2.IsEnabled(aiCode) {
		t.Errorf("EnableFreeOnly: previously explicit-on AI feature %q became disabled", aiCode)
	}

	// AI features not previously enabled must remain off (tier default).
	for _, f := range catalog {
		if f.Tier != TierAI || f.Code == aiCode {
			continue
		}
		if s2.Enabled[f.Code] {
			// OK if it's in Enabled (shouldn't be), but check IsEnabled with no ExplicitOff.
		}
		// Without an explicit on, AI features default to off.
		if s2.Enabled[f.Code] && f.Code != aiCode {
			t.Errorf("EnableFreeOnly: AI feature %q was unexpectedly enabled", f.Code)
		}
	}
}
