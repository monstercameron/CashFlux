package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestIdResolver covers all three resolution branches: an exact id passes
// through, a (case-insensitive) name resolves to its id, and an unknown value is
// returned unchanged.
func TestIdResolver(t *testing.T) {
	resolve := idResolver([][2]string{{"a1", "Checking"}, {"a2", "Savings"}})
	cases := map[string]string{
		"a1":       "a1",      // exact id passthrough
		"Checking": "a1",      // name match
		"savings":  "a2",      // case-insensitive name match
		"unknown":  "unknown", // unresolved passthrough
		"":         "",        // empty passthrough
	}
	for in, want := range cases {
		if got := resolve(in); got != want {
			t.Errorf("resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFreshnessWindowsOverrides covers the per-type override loop: a household
// override is layered over the built-in default windows.
func TestFreshnessWindowsOverrides(t *testing.T) {
	a := newApp(t, false)
	s := a.Settings()
	if s.FreshnessOverrides == nil {
		s.FreshnessOverrides = map[string]int{}
	}
	s.FreshnessOverrides["checking"] = 5
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	w := a.FreshnessWindows()
	if w[domain.TypeChecking] != 5 {
		t.Errorf("checking window = %d, want 5 (override applied over the default)", w[domain.TypeChecking])
	}
}

// TestBudgetMethodologyIsHouseholdOnly documents the current config layering:
// methodology defaults at read time, then the household Settings value wins.
// Members do not carry a methodology override today.
func TestBudgetMethodologyIsHouseholdOnly(t *testing.T) {
	a := newApp(t, false)
	if got := budgeting.ParseMethodology(a.Settings().BudgetMethodology); got != budgeting.MethodSimple {
		t.Errorf("default methodology = %q, want simple", got)
	}
	s := a.Settings()
	s.BudgetMethodology = string(budgeting.MethodZeroBased)
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if err := a.PutMember(domain.Member{ID: "m1", Name: "Alex"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	if got := budgeting.ParseMethodology(a.Settings().BudgetMethodology); got != budgeting.MethodZeroBased {
		t.Errorf("household methodology = %q, want zero-based", got)
	}
}
