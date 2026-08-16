// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMethodologyValid(t *testing.T) {
	for _, m := range []Methodology{MethodSimple, MethodZeroBased, MethodEnvelope} {
		if !m.Valid() {
			t.Errorf("%q should be valid", m)
		}
	}
	if Methodology("nonsense").Valid() {
		t.Error(`"nonsense" should be invalid`)
	}
}

func TestParseMethodology(t *testing.T) {
	cases := map[string]Methodology{
		"":           MethodSimple, // unset → safe default
		"simple":     MethodSimple,
		"zero-based": MethodZeroBased,
		"envelope":   MethodEnvelope,
		"bogus":      MethodSimple, // unknown → safe default
	}
	for in, want := range cases {
		if got := ParseMethodology(in); got != want {
			t.Errorf("ParseMethodology(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToAssign(t *testing.T) {
	tests := []struct {
		name                  string
		income, totalBudgeted int64
		want                  int64
	}{
		{"under-assigned leaves a remainder", 500000, 300000, 200000},
		{"fully assigned is zero", 400000, 400000, 0},
		{"over-assigned is negative", 300000, 450000, -150000},
		{"no income", 0, 100000, -100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToAssign(tt.income, tt.totalBudgeted); got != tt.want {
				t.Errorf("ToAssign(%d, %d) = %d, want %d", tt.income, tt.totalBudgeted, got, tt.want)
			}
		})
	}
}

// --- C590: a global method change must say what it will and will not touch ---

func TestMethodChangeImpactSplitsFollowersFromOverriders(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "a"},                  // follows the household method
		{ID: "b", Methodology: ""}, // same, explicitly empty
		{ID: "c", Methodology: string(MethodZeroBased)},     // overrides
		{ID: "d", Methodology: string(MethodEnvelope)},      // overrides
		{ID: "e", Methodology: string(MethodZeroBased)},     // overrides
		{ID: "f", Methodology: "something-from-the-future"}, // unknown → follows, like ParseMethodology
	}

	got := MethodChangeImpact(budgets)
	if got.Following != 3 {
		t.Errorf("Following = %d, want 3 (two empty plus the unknown value)", got.Following)
	}
	if got.Overriding != 3 {
		t.Errorf("Overriding = %d, want 3", got.Overriding)
	}
	if got.Total() != len(budgets) {
		t.Errorf("Total = %d, want %d — every budget must land on one side", got.Total(), len(budgets))
	}
	if got.Overrides[MethodZeroBased] != 2 || got.Overrides[MethodEnvelope] != 1 {
		t.Errorf("Overrides = %v, want 2 zero-based and 1 envelope", got.Overrides)
	}
}

// The rule has to match ParseMethodology, or the preview would promise a change
// to budgets that behave as the default anyway.
func TestMethodChangeImpactAgreesWithParseMethodology(t *testing.T) {
	for _, raw := range []string{"", "nonsense", "SIMPLE"} {
		impact := MethodChangeImpact([]domain.Budget{{ID: "x", Methodology: raw}})
		parsed := ParseMethodology(raw)
		if impact.Following != 1 {
			t.Errorf("%q counted as an override, but ParseMethodology resolves it to %q (the default)", raw, parsed)
		}
	}
}

func TestMethodChangeImpactEmpty(t *testing.T) {
	got := MethodChangeImpact(nil)
	if got.Total() != 0 || len(got.Overrides) != 0 {
		t.Errorf("MethodChangeImpact(nil) = %+v, want an empty impact", got)
	}
}
