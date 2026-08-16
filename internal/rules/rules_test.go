// SPDX-License-Identifier: MIT

package rules

import (
	"testing"
	"time"
)

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

// ─── C372: a rule's durable record of what it has done ───────────────────────

func TestRecordHitsAccumulatesAndAdvancesTheDate(t *testing.T) {
	t1 := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	r := Rule{ID: "r1"}.RecordHits(3, t1)
	if r.HitCount != 3 || !r.LastRunAt.Equal(t1) {
		t.Fatalf("first credit = %d @ %s", r.HitCount, r.LastRunAt)
	}
	r = r.RecordHits(2, t2)
	if r.HitCount != 5 {
		t.Errorf("HitCount = %d, want 5 — credits accumulate", r.HitCount)
	}
	if !r.LastRunAt.Equal(t2) {
		t.Errorf("LastRunAt = %s, want %s", r.LastRunAt, t2)
	}
	// An out-of-order credit (a backfill over old data) adds to the count but
	// must not drag "last fired" backwards.
	r = r.RecordHits(1, t1)
	if r.HitCount != 6 {
		t.Errorf("HitCount = %d, want 6", r.HitCount)
	}
	if !r.LastRunAt.Equal(t2) {
		t.Errorf("LastRunAt moved backwards to %s", r.LastRunAt)
	}
}

// A zero or negative credit is a no-op, so a caller can hand it a tally without
// checking first.
func TestRecordHitsIgnoresNothingToRecord(t *testing.T) {
	at := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	base := Rule{ID: "r1", HitCount: 4, LastRunAt: at}
	for _, n := range []int{0, -1} {
		got := base.RecordHits(n, at.AddDate(0, 1, 0))
		if got.HitCount != base.HitCount || !got.LastRunAt.Equal(base.LastRunAt) {
			t.Errorf("RecordHits(%d) changed the rule: %d @ %s", n, got.HitCount, got.LastRunAt)
		}
	}
}
