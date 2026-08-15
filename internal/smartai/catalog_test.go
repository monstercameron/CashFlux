// SPDX-License-Identifier: MIT

package smartai

import "testing"

func testCatalog() Catalog {
	return NewCatalog([]Cat{
		{ID: "c-groceries", Path: "Groceries"},
		{ID: "c-auto-gas", Path: "Auto > Gas"},
		{ID: "c-util-gas", Path: "Utilities > Gas"},
		{ID: "c-salary", Path: "Salary", Income: true},
	})
}

// TestCatalogResolvesTheCollision is C489: two parents own a child called "Gas",
// and a name-keyed resolver would silently pick whichever landed last.
func TestCatalogResolvesTheCollision(t *testing.T) {
	c := testCatalog()
	auto, ok := c.Lookup("Auto > Gas")
	if !ok || auto.ID != "c-auto-gas" {
		t.Errorf("Auto > Gas resolved to %+v, want c-auto-gas", auto)
	}
	util, ok := c.Lookup("Utilities > Gas")
	if !ok || util.ID != "c-util-gas" {
		t.Errorf("Utilities > Gas resolved to %+v, want c-util-gas", util)
	}
	// A bare leaf must NOT resolve: it is genuinely ambiguous, and guessing
	// would reintroduce exactly the bug this replaces.
	if got, ok := c.Lookup("Gas"); ok {
		t.Errorf("bare %q resolved to %+v; an ambiguous leaf must not resolve", "Gas", got)
	}
}

func TestCatalogLookupTolerantOfFormatting(t *testing.T) {
	c := testCatalog()
	for _, in := range []string{
		"Auto > Gas", "auto > gas", "  Auto>Gas  ", "Auto › Gas", "\"Auto > Gas\"", "Auto  >  Gas",
	} {
		got, ok := c.Lookup(in)
		if !ok || got.ID != "c-auto-gas" {
			t.Errorf("Lookup(%q) = %+v ok=%v, want c-auto-gas", in, got, ok)
		}
	}
	if _, ok := c.Lookup(""); ok {
		t.Error("empty path must not resolve")
	}
}

func TestCatalogPromptCarriesPathAndKind(t *testing.T) {
	got := testCatalog().Prompt()
	for _, want := range []string{
		"Auto > Gas | expense\n", "Utilities > Gas | expense\n", "Salary | income\n",
	} {
		if !contains(got, want) {
			t.Errorf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestCatalogDropsBlanksAndDuplicates(t *testing.T) {
	c := NewCatalog([]Cat{
		{ID: "a", Path: "Groceries"},
		{ID: "b", Path: "Groceries"}, // duplicate path: first wins
		{ID: "", Path: "No ID"},
		{ID: "d", Path: "   "},
	})
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
	if got, _ := c.Lookup("Groceries"); got.ID != "a" {
		t.Errorf("duplicate path resolved to %q, want the first definition 'a'", got.ID)
	}
}

// TestParseConfidence is C488: the contract can now express doubt instead of
// signalling it by silence.
func TestParseConfidence(t *testing.T) {
	cat := testCatalog()
	answer := "" +
		"1 => Groceries | high\n" +
		"2 => Auto > Gas | low\n" +
		"3 => Utilities > Gas | medium\n" +
		"4 => Groceries\n" // no marker → medium
	got := ParseCategoryAssignments(answer, 4, cat)
	if len(got) != 4 {
		t.Fatalf("got %d assignments, want 4: %+v", len(got), got)
	}
	want := []Confidence{ConfHigh, ConfLow, ConfMedium, ConfMedium}
	for i, w := range want {
		if got[i].Confidence != w {
			t.Errorf("assignment %d confidence = %v, want %v", i+1, got[i].Confidence, w)
		}
	}
	if got[1].CategoryID != "c-auto-gas" {
		t.Errorf("path with a confidence marker mis-resolved: %+v", got[1])
	}
}

func TestAtLeastFiltersByConfidence(t *testing.T) {
	in := []CategoryAssignment{
		{Ref: 1, Confidence: ConfHigh}, {Ref: 2, Confidence: ConfLow}, {Ref: 3, Confidence: ConfMedium},
	}
	if got := AtLeast(in, ConfHigh); len(got) != 1 || got[0].Ref != 1 {
		t.Errorf("AtLeast(high) = %+v, want just ref 1", got)
	}
	if got := AtLeast(in, ConfMedium); len(got) != 2 {
		t.Errorf("AtLeast(medium) returned %d, want 2", len(got))
	}
	if got := AtLeast(in, ConfLow); len(got) != 3 {
		t.Errorf("AtLeast(low) returned %d, want 3", len(got))
	}
}

// TestRejectSignMismatches is C490: an income category must never land on an
// expense charge, or the reverse.
func TestRejectSignMismatches(t *testing.T) {
	cat := testCatalog()
	// Ref 1 is an expense charge; the model handed it the income category.
	answer := "1 => Salary | high\n2 => Groceries | high\n"
	parsed := ParseCategoryAssignments(answer, 2, cat)
	if len(parsed) != 2 {
		t.Fatalf("parse returned %d, want 2", len(parsed))
	}
	got := RejectSignMismatches(parsed, map[int]bool{1: false, 2: false})
	if len(got) != 1 || got[0].Ref != 2 {
		t.Errorf("RejectSignMismatches = %+v, want only ref 2", got)
	}

	// An income charge correctly given the income category survives.
	got = RejectSignMismatches(parsed, map[int]bool{1: true, 2: false})
	if len(got) != 2 {
		t.Errorf("correctly-signed assignments were dropped: %+v", got)
	}

	// A ref with no sign information is left alone.
	got = RejectSignMismatches(parsed, map[int]bool{})
	if len(got) != 2 {
		t.Errorf("unknown refs must not be dropped: %+v", got)
	}
}

// TestParseCategorySuggestionsParent is C491.
func TestParseCategorySuggestionsParent(t *testing.T) {
	existing := map[string]bool{"shopping": true, "auto": true}
	answer := "" +
		"Household | expense | Shopping\n" + // nests under an existing parent
		"Tolls | expense | Auto\n" +
		"Hobbies | expense\n" + // top level
		"Bogus | expense | Nonexistent\n" // parent does not exist → top level
	got := ParseCategorySuggestions(answer, existing)
	if len(got) != 4 {
		t.Fatalf("got %d suggestions, want 4: %+v", len(got), got)
	}
	want := []string{"Shopping", "Auto", "", ""}
	for i, w := range want {
		if got[i].ParentName != w {
			t.Errorf("suggestion %q parent = %q, want %q", got[i].Name, got[i].ParentName, w)
		}
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
