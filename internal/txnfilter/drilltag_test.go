// SPDX-License-Identifier: MIT

package txnfilter

import "testing"

// C651: "Filter to #coffee" on a row left the search text, the chip row, the
// count and the ledger unchanged.
//
// On a view already narrowed to one matching row the COUNT genuinely cannot move
// — the one row that matches the search is the one carrying the tag — so the
// evidence that the action landed has to be the removable chip. These pin that
// contract: the drill adds a tag criterion, keeps everything else, shows up as a
// chip, and comes off on its own.

func TestDrillToTagAddsAChipWithoutTouchingTheSearch(t *testing.T) {
	c := Criteria{Text: "UXJ-S01"}.Normalize().DrillToTag("coffee")

	if c.Text != "UXJ-S01" {
		t.Errorf("Text = %q — a tag drill-down must ADD a criterion, not replace the query", c.Text)
	}
	found := false
	for _, af := range c.ActiveFilters() {
		if af.Field == FieldTag && af.Value == "coffee" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no removable Tag chip after drilling to #coffee; chips = %+v (C651)", c.ActiveFilters())
	}
}

// TestDrillToTagChipRemovesOnlyItself is the "clear way back" half of the AC.
func TestDrillToTagChipRemovesOnlyItself(t *testing.T) {
	c := Criteria{Text: "UXJ-S01", Member: "m1"}.Normalize().DrillToTag("coffee")
	back := c.RemoveValue(FieldTag, "coffee")

	if back.Text != "UXJ-S01" || back.Member != "m1" {
		t.Errorf("removing the tag chip took other filters with it: text=%q member=%q", back.Text, back.Member)
	}
	for _, af := range back.ActiveFilters() {
		if af.Field == FieldTag {
			t.Errorf("Tag filter survived its own chip removal: %+v", af)
		}
	}
}

// TestDrillToTagReplacesAnEarlierSelection: the control drills to ONE tag, so a
// second click must not accumulate an AND of two tags that matches nothing.
func TestDrillToTagReplacesAnEarlierSelection(t *testing.T) {
	c := Criteria{Tags: "travel,food"}.Normalize().DrillToTag("coffee")
	vals := c.SelectedValues(FieldTag)
	if len(vals) != 1 || vals[0] != "coffee" {
		t.Errorf("selected tags = %v, want exactly [coffee]", vals)
	}
}

// TestDrillToTagIsAScopeChange guards the feedback path: the ledger announces the
// drill only when the criteria actually moved, so ScopeChanged has to see it.
func TestDrillToTagIsAScopeChange(t *testing.T) {
	before := Criteria{Text: "UXJ-S01"}.Normalize()
	after := before.DrillToTag("coffee")
	if !ScopeChanged(before, after) {
		t.Error("drilling to a tag did not register as a scope change — the ledger would neither reset the page nor announce it (C651)")
	}
	if ScopeChanged(after, after.DrillToTag("coffee")) {
		t.Error("re-drilling to the SAME tag registered as a change — the ledger would claim it narrowed something twice")
	}
}

// TestDrillToTagIgnoresABlankTag: clearing the user's tag filter is not a
// reasonable guess at what an empty chip meant.
func TestDrillToTagIgnoresABlankTag(t *testing.T) {
	c := Criteria{Tag: "coffee"}.Normalize()
	if got := c.DrillToTag("  "); got.Tag != "coffee" {
		t.Errorf("Tag = %q after drilling to a blank tag, want it untouched", got.Tag)
	}
}

// TestDrillToTagDropsScopeAny keeps the single-tag meaning exact: ScopeAny widens
// a tag filter to "any of these", and a drill-down to one tag has nothing to
// widen.
func TestDrillToTagDropsScopeAny(t *testing.T) {
	c := Criteria{Tags: "a,b", ScopeAny: true}.Normalize().DrillToTag("coffee")
	if c.ScopeAny {
		t.Error("ScopeAny survived a single-tag drill-down")
	}
}
