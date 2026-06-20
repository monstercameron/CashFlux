package rules

import "testing"

func TestRuleMatchCount(t *testing.T) {
	texts := []string{"Morning coffee", "tea time", "COFFEE SHOP downtown", "lunch", "coffee beans"}
	tests := []struct {
		match string
		want  int
	}{
		{"coffee", 3},    // case-insensitive substring across 3 entries
		{"  Coffee ", 3}, // trimmed, still case-insensitive
		{"tea", 1},
		{"zzz", 0},
		{"", 0}, // an empty match phrase matches nothing
	}
	for _, tc := range tests {
		if got := (Rule{Match: tc.match}).MatchCount(texts); got != tc.want {
			t.Errorf("MatchCount(%q) = %d, want %d", tc.match, got, tc.want)
		}
	}
}

func TestCoveredAndUncovered(t *testing.T) {
	rs := []Rule{
		{ID: "r1", Match: "coffee", SetCategoryID: "dining"},
		{ID: "r2", Match: "uber", SetCategoryID: "transport"},
	}
	texts := []string{"coffee run", "Uber ride home", "grocery store", "another coffee", "rent"}
	if got := Covered(rs, texts); got != 3 {
		t.Errorf("Covered = %d, want 3", got)
	}
	if got := Uncovered(rs, texts); got != 2 {
		t.Errorf("Uncovered = %d, want 2", got)
	}
	// No rules → nothing covered.
	if got := Covered(nil, texts); got != 0 {
		t.Errorf("Covered(nil) = %d, want 0", got)
	}
	if got := Uncovered(nil, texts); got != len(texts) {
		t.Errorf("Uncovered(nil) = %d, want %d", got, len(texts))
	}
}
