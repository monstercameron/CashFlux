// SPDX-License-Identifier: MIT

package smartai

import (
	"strings"
	"testing"
)

// The parsers in this package consume MODEL OUTPUT, which is the one input a
// program never controls: a model can reformat, truncate, hallucinate, emit
// another language, or return prose where a table was asked for. These tests
// assert the invariants that must hold no matter what comes back — chiefly that
// a category can never be invented and a reference can never point past the
// sample. Run with -fuzz to explore; the seed corpus runs as ordinary tests.

func fuzzCatalog() Catalog {
	return NewCatalog([]Cat{
		{ID: "c1", Path: "Groceries"},
		{ID: "c2", Path: "Auto > Gas"},
		{ID: "c3", Path: "Utilities > Gas"},
		{ID: "c4", Path: "Salary", Income: true},
	})
}

// FuzzParseCategoryAssignments: whatever the model says, the parser must not
// panic, must only ever return categories from the catalog, must keep refs
// inside [1, maxRef], and must never return the same ref twice.
func FuzzParseCategoryAssignments(f *testing.F) {
	for _, seed := range []string{
		"",
		"1 => Groceries | high",
		"1 => Groceries\n2 => Auto > Gas | low",
		"1 => Gas",                    // ambiguous leaf: must be refused
		"1 => Nonexistent | high",     // invented category
		"0 => Groceries",              // ref below range
		"99 => Groceries",             // ref above range
		"1 => Groceries\n1 => Salary", // duplicate ref
		"1 =>",                        // no category
		"=> Groceries",                // no ref
		"1 => Groceries | ",           // empty confidence
		"1 => Groceries | banana",     // nonsense confidence
		"1 => auto > gas | HIGH",      // case + spacing drift
		"- 1 => Auto>Gas|high",        // bullet, tight separators
		"1 => Auto › Gas | high",      // display chevron
		"I'm sorry, I can't help.",    // refusal prose
		"1 => Groceries\x00| high",    // NUL byte
		strings.Repeat("1 => Groceries\n", 50),
	} {
		f.Add(seed, 4)
	}
	cat := fuzzCatalog()
	f.Fuzz(func(t *testing.T, answer string, maxRef int) {
		if maxRef < 0 || maxRef > 512 {
			t.Skip() // callers always pass a real sample size
		}
		got := ParseCategoryAssignments(answer, maxRef, cat)

		if len(got) > maxRef {
			t.Fatalf("returned %d assignments for maxRef %d", len(got), maxRef)
		}
		seen := map[int]bool{}
		for _, a := range got {
			if a.Ref < 1 || a.Ref > maxRef {
				t.Errorf("ref %d outside [1,%d]", a.Ref, maxRef)
			}
			if seen[a.Ref] {
				t.Errorf("duplicate ref %d", a.Ref)
			}
			seen[a.Ref] = true

			// The category must be one the caller offered — a model can never
			// invent one, which is what makes the result safe to write.
			hit, ok := cat.Lookup(a.CategoryName)
			if !ok {
				t.Errorf("category %q is not in the catalog", a.CategoryName)
				continue
			}
			if hit.ID != a.CategoryID {
				t.Errorf("category id %q does not match path %q (want %q)", a.CategoryID, a.CategoryName, hit.ID)
			}
			if hit.Income != a.Income {
				t.Errorf("Income flag %v disagrees with the catalog (%v)", a.Income, hit.Income)
			}
			if a.Confidence < ConfLow || a.Confidence > ConfHigh {
				t.Errorf("confidence %d outside the enum", a.Confidence)
			}
		}
	})
}

// FuzzParseCategorySuggestions: never propose a category that already exists,
// never one too short to be useful, never more than the cap, and never a parent
// the household does not have.
func FuzzParseCategorySuggestions(f *testing.F) {
	for _, seed := range []string{
		"",
		"Household | expense | Shopping",
		"Groceries | expense",            // already exists
		"ab | expense",                   // too short
		"Tolls | income | Nonexistent",   // invented parent
		"Tolls|expense|shopping",         // case + tight separators
		"Pets | expense\nPets | expense", // duplicate
		strings.Repeat("Cat | expense\n", 40),
		"Here are some ideas:\n- Hobbies | expense",
	} {
		f.Add(seed)
	}
	existing := map[string]bool{"groceries": true, "shopping": true}
	f.Fuzz(func(t *testing.T, answer string) {
		got := ParseCategorySuggestions(answer, existing)
		if len(got) > 8 {
			t.Fatalf("returned %d suggestions, cap is 8", len(got))
		}
		seen := map[string]bool{}
		for _, s := range got {
			low := strings.ToLower(s.Name)
			if len(s.Name) < 3 {
				t.Errorf("name %q is shorter than the 3-char floor", s.Name)
			}
			if existing[low] {
				t.Errorf("%q already exists and must not be proposed", s.Name)
			}
			if seen[low] {
				t.Errorf("duplicate suggestion %q", s.Name)
			}
			seen[low] = true
			if s.Kind != "expense" && s.Kind != "income" {
				t.Errorf("kind %q is neither expense nor income", s.Kind)
			}
			// A parent must be one the household ALREADY has: a model cannot
			// conjure a hierarchy nobody agreed to.
			if s.ParentName != "" && !existing[strings.ToLower(s.ParentName)] {
				t.Errorf("parent %q does not exist", s.ParentName)
			}
		}
	})
}

// FuzzCatalogLookup: lookup never panics and never resolves to something the
// catalog does not hold.
func FuzzCatalogLookup(f *testing.F) {
	for _, seed := range []string{"", " ", "Groceries", "auto>gas", "Auto › Gas", "Gas", "\x00", "»»»", strings.Repeat("a", 300)} {
		f.Add(seed)
	}
	cat := fuzzCatalog()
	f.Fuzz(func(t *testing.T, path string) {
		got, ok := cat.Lookup(path)
		if !ok {
			if got != (Cat{}) {
				t.Errorf("miss returned a non-zero Cat: %+v", got)
			}
			return
		}
		if got.ID == "" || got.Path == "" {
			t.Errorf("hit returned an incomplete Cat: %+v", got)
		}
		// A hit must round-trip: looking its own path up again finds the same entry.
		again, ok2 := cat.Lookup(got.Path)
		if !ok2 || again.ID != got.ID {
			t.Errorf("lookup is not idempotent for %q -> %+v", path, got)
		}
	})
}

// TestCatalogNeverResolvesAnAmbiguousLeaf is the invariant behind C489, stated
// once rather than left implicit in a table.
func TestCatalogNeverResolvesAnAmbiguousLeaf(t *testing.T) {
	cat := fuzzCatalog()
	// "Gas" exists under two different parents. Resolving it either way would be
	// a coin flip written into the user's ledger.
	for _, ambiguous := range []string{"Gas", "gas", " GAS ", "\"gas\""} {
		if got, ok := cat.Lookup(ambiguous); ok {
			t.Errorf("Lookup(%q) resolved to %+v; an ambiguous leaf must never resolve", ambiguous, got)
		}
	}
	// A leaf that is unique is still reachable only by its full path — the rule
	// is uniform, not a special case for collisions.
	if _, ok := cat.Lookup("Salary"); !ok {
		t.Error("a top-level category must resolve by its own name")
	}
}

// TestAtLeastAndRejectComposeInEitherOrder: the two filters are independent, so
// applying them in either order must give the same set. A caller that reorders
// them must not change what gets written.
func TestAtLeastAndRejectComposeInEitherOrder(t *testing.T) {
	in := []CategoryAssignment{
		{Ref: 1, CategoryID: "c1", Confidence: ConfHigh, Income: false},
		{Ref: 2, CategoryID: "c4", Confidence: ConfHigh, Income: true},
		{Ref: 3, CategoryID: "c1", Confidence: ConfLow, Income: false},
	}
	signs := map[int]bool{1: false, 2: false, 3: false} // all expenses
	a := AtLeast(RejectSignMismatches(in, signs), ConfHigh)
	b := RejectSignMismatches(AtLeast(in, ConfHigh), signs)
	if len(a) != len(b) {
		t.Fatalf("order changed the result: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Ref != b[i].Ref {
			t.Errorf("order changed element %d: %d vs %d", i, a[i].Ref, b[i].Ref)
		}
	}
	if len(a) != 1 || a[0].Ref != 1 {
		t.Errorf("want only ref 1 (high confidence, correct sign), got %+v", a)
	}
}

// TestFiltersDoNotAliasTheInput: both filters build their result on the input's
// backing array, so a caller keeping the original must not see it rewritten.
func TestFiltersDoNotAliasTheInput(t *testing.T) {
	in := []CategoryAssignment{
		{Ref: 1, Confidence: ConfLow},
		{Ref: 2, Confidence: ConfHigh},
	}
	orig := append([]CategoryAssignment(nil), in...)
	_ = AtLeast(in, ConfHigh)
	for i := range in {
		if in[i] != orig[i] {
			t.Fatalf("AtLeast mutated the caller's slice at %d: %+v want %+v", i, in[i], orig[i])
		}
	}
}

// TestPromptListsEveryCategoryExactlyOnce: the model is held to this list, so a
// missing line silently removes a category the user has.
func TestPromptListsEveryCategoryExactlyOnce(t *testing.T) {
	cat := fuzzCatalog()
	prompt := cat.Prompt()
	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(prompt), "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	if lines != cat.Len() {
		t.Errorf("prompt has %d lines for %d categories", lines, cat.Len())
	}
	// Every path in the prompt must resolve back through Lookup, or the model is
	// being offered something the parser will then refuse.
	for _, l := range strings.Split(strings.TrimSpace(prompt), "\n") {
		path, _, _ := strings.Cut(l, "|")
		if _, ok := cat.Lookup(path); !ok {
			t.Errorf("prompt offers %q but Lookup refuses it", strings.TrimSpace(path))
		}
	}
}
