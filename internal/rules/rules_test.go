// SPDX-License-Identifier: MIT

package rules

import "testing"

func sampleRules() []Rule {
	return []Rule{
		{ID: "r1", Match: "coffee", SetCategoryID: "cafe", SetTags: []string{"treats"}},
		{ID: "r2", Match: "uber", SetCategoryID: "transport"},
		{ID: "r3", Match: "", SetCategoryID: "ignored"}, // empty match never fires
	}
}

func TestCategoryMatch(t *testing.T) {
	rs := sampleRules()
	if got := Category(rs, "Starbucks", "Morning COFFEE"); got != "cafe" {
		t.Errorf("Category = %q, want cafe (case-insensitive)", got)
	}
	if got := Category(rs, "Uber trip", ""); got != "transport" {
		t.Errorf("Category = %q, want transport", got)
	}
	if got := Category(rs, "Grocery store", "veg"); got != "" {
		t.Errorf("Category = %q, want empty (no match)", got)
	}
}

func TestFirstMatchWins(t *testing.T) {
	rs := []Rule{
		{ID: "a", Match: "shop", SetCategoryID: "first"},
		{ID: "b", Match: "coffee shop", SetCategoryID: "second"},
	}
	r := FirstMatch(rs, "the coffee shop")
	if r == nil || r.ID != "a" {
		t.Errorf("FirstMatch = %v, want rule a (first in order)", r)
	}
}

func TestTags(t *testing.T) {
	rs := sampleRules()
	tags := Tags(rs, "Coffee Bean", "")
	if len(tags) != 1 || tags[0] != "treats" {
		t.Errorf("Tags = %v, want [treats]", tags)
	}
	if Tags(rs, "Uber", "") != nil {
		t.Error("rule without tags should yield nil tags")
	}
}

func TestEmptyMatchNeverFires(t *testing.T) {
	if FirstMatch([]Rule{{Match: ""}}, "anything") != nil {
		t.Error("empty Match should never fire")
	}
}

// TestFirstMatchIgnoresCurrentCategory verifies that FirstMatch (which drives the
// backfill path) returns a rule regardless of whether the transaction already has a
// category. The caller — not this package — decides whether to overwrite; the match
// itself must never be suppressed by an existing category. This is the correctness
// guarantee that makes rule corrections propagate to past transactions (C108).
func TestFirstMatchIgnoresCurrentCategory(t *testing.T) {
	rs := []Rule{
		{ID: "r1", Match: "uber", SetCategoryID: "transport"},
	}
	// Simulate a transaction that already carries "food" — the match still fires.
	r := FirstMatch(rs, "Uber Eats dinner")
	if r == nil {
		t.Fatal("FirstMatch returned nil; expected r1 to match")
	}
	if r.SetCategoryID != "transport" {
		t.Errorf("SetCategoryID = %q, want transport", r.SetCategoryID)
	}
	// A transaction whose text doesn't match returns nil regardless.
	if got := FirstMatch(rs, "Grocery store lunch"); got != nil {
		t.Errorf("non-matching text should return nil, got %+v", got)
	}
}

func TestConflicts(t *testing.T) {
	rs := []Rule{
		{ID: "a", Match: "shop"},        // fires
		{ID: "b", Match: "coffee shop"}, // shadowed by a (contains "shop")
		{ID: "c", Match: "uber"},        // fires
		{ID: "d", Match: "COFFEE Shop"}, // shadowed by a too (case-insensitive)
		{ID: "e", Match: ""},            // dead: no match phrase
	}
	got := Conflicts(rs)
	want := []Conflict{
		{Index: 1, ShadowedBy: 0},
		{Index: 3, ShadowedBy: 0},
		{Index: 4, ShadowedBy: -1},
	}
	if len(got) != len(want) {
		t.Fatalf("Conflicts = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Conflicts[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// No conflicts among distinct, non-overlapping phrases.
	if c := Conflicts([]Rule{{Match: "uber"}, {Match: "lyft"}}); len(c) != 0 {
		t.Errorf("expected no conflicts, got %+v", c)
	}
}
