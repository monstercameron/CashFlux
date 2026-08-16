// SPDX-License-Identifier: MIT

package catname

import (
	"sort"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"Groceries", "Groceries"},
		{"  Groceries  ", "Groceries"},
		{"Eating  Out", "Eating Out"},   // the double-space typo is the same name
		{"Eating\tOut", "Eating Out"},   // so is a tab
		{"Eating\n Out ", "Eating Out"}, // and a newline from a paste
		{"CASE", "CASE"},                // Normalize does not fold case
		{"a  b   c", "a b c"},
		{" Gas ", "Gas"}, // non-breaking space is whitespace too
	}
	for _, tc := range cases {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEqualNames(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Groceries", "groceries", true},
		{"Groceries", "GROCERIES", true},
		{" Groceries ", "Groceries", true},
		{"Eating  Out", "Eating Out", true},
		{"Groceries", "Grocery", false},
		{"", "", false},    // an empty name is never equal to anything, including itself
		{"", "Gas", false}, // ...so a blank never collides
		{"  ", "", false},
	}
	for _, tc := range cases {
		if got := EqualNames(tc.a, tc.b); got != tc.want {
			t.Errorf("EqualNames(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestLessNatural is the C518 requirement: lists read the way a person expects,
// which plain lexicographic order does not deliver once numbers are involved.
func TestLessNatural(t *testing.T) {
	in := []string{"Item 10", "Item 9", "Item 1", "item 2", "Alpha", "zeta", "Item 100"}
	want := []string{"Alpha", "Item 1", "item 2", "Item 9", "Item 10", "Item 100", "zeta"}
	got := append([]string(nil), in...)
	sort.SliceStable(got, func(i, j int) bool { return Less(got[i], got[j]) })
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("natural sort = %v, want %v", got, want)
		}
	}
}

func TestLessEdgeCases(t *testing.T) {
	cases := []struct {
		a, b string
		want bool // Less(a,b)
	}{
		{"Item 2", "Item 10", true},
		{"Item 10", "Item 2", false},
		{"2", "10", true},
		{"a", "b", true},
		{"a", "A", false}, // equal ignoring case; the case tiebreak puts "A" first
		{"A", "a", true},  // ...and it is a total order, never both-directions-true
		{"", "a", true},   // empty sorts first
		{"a", "", false},
		{"Item 007", "Item 7", true}, // same value, more leading zeros first
		{"Item 7", "Item 007", false},
		// A digit run far longer than int64 must not overflow or panic.
		{"n 99999999999999999999999999", "n 999999999999999999999999999", true},
		{"n 999999999999999999999999999", "n 99999999999999999999999999", false},
		{"Gas", "Gas Station", true}, // a prefix sorts before the longer name
		{"Item 1a", "Item 1b", true}, // digits then letters
	}
	for _, tc := range cases {
		if got := Less(tc.a, tc.b); got != tc.want {
			t.Errorf("Less(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestLessIsATotalOrder guards the property that matters to sort.Slice: for any
// pair, Less must not report both orders as true, or the sort is undefined.
func TestLessIsATotalOrder(t *testing.T) {
	names := []string{"", " ", "a", "A", "Item 1", "Item 01", "Item 10", "item 10", "10", "9", "Zz", "gas", "Gas"}
	for _, a := range names {
		for _, b := range names {
			if Less(a, b) && Less(b, a) {
				t.Errorf("Less is not antisymmetric for (%q,%q)", a, b)
			}
			if a == b && Less(a, b) {
				t.Errorf("Less(%q,%q) must be false for identical inputs", a, b)
			}
		}
	}
}

func cat(id, name, parent string, kind domain.CategoryKind) domain.Category {
	return domain.Category{ID: id, Name: name, ParentID: parent, Kind: kind}
}

// TestFindSiblingIsParentAware is C536(c): a same-named child must NOT satisfy a
// request for a top-level category. Resolving across the whole tree is how the
// add-a-budget form silently attached budgets to the wrong category.
func TestFindSiblingIsParentAware(t *testing.T) {
	cats := []domain.Category{
		cat("auto", "Auto", "", domain.KindExpense),
		cat("auto-gas", "Gas", "auto", domain.KindExpense),
		cat("food", "Food", "", domain.KindExpense),
	}
	if _, ok := FindSibling(cats, "", "Gas"); ok {
		t.Error("a top-level Gas must not resolve to the Gas nested under Auto")
	}
	got, ok := FindSibling(cats, "auto", "gas")
	if !ok || got.ID != "auto-gas" {
		t.Errorf("FindSibling(auto,gas) = %+v,%v; want auto-gas", got, ok)
	}
	if _, ok := FindSibling(cats, "", ""); ok {
		t.Error("a blank name must never resolve")
	}
	if _, ok := FindSibling(cats, "", "Nothing"); ok {
		t.Error("an absent name must not resolve")
	}
}

// TestFindSiblingIgnoresKind is C536(b): an income "Investments" and an expense
// "Investments" are the same name to the person reading the list, so the second
// one must be found rather than minted.
func TestFindSiblingIgnoresKind(t *testing.T) {
	cats := []domain.Category{cat("inv", "Investments", "", domain.KindIncome)}
	got, ok := FindSibling(cats, "", "investments")
	if !ok || got.ID != "inv" {
		t.Errorf("an income category must still be found by name; got %+v,%v", got, ok)
	}
}

func TestCollision(t *testing.T) {
	cats := []domain.Category{
		cat("a", "Groceries", "", domain.KindExpense),
		cat("auto", "Auto", "", domain.KindExpense),
		cat("b", "Gas", "auto", domain.KindExpense),
	}
	if _, ok := Collision(cats, cat("new", "groceries", "", domain.KindExpense)); !ok {
		t.Error("a new category taking an existing sibling name must collide")
	}
	if _, ok := Collision(cats, cat("a", "Groceries", "", domain.KindExpense)); ok {
		t.Error("re-saving a category must not collide with itself")
	}
	if _, ok := Collision(cats, cat("new", "Gas", "", domain.KindExpense)); ok {
		t.Error("a top-level Gas must not collide with the Gas under Auto — different parents")
	}
	if _, ok := Collision(cats, cat("new", "Gas", "auto", domain.KindExpense)); !ok {
		t.Error("a second Gas under Auto must collide")
	}
	if _, ok := Collision(cats, cat("new", "  ", "", domain.KindExpense)); ok {
		t.Error("a blank name must not be reported as a collision (ValidateCategory rejects it)")
	}
	// Kind must not excuse a collision.
	if _, ok := Collision(cats, cat("new", "Groceries", "", domain.KindIncome)); !ok {
		t.Error("a same-named income category must still collide with the expense one")
	}
}

// TestNameChangedProtectsExistingDuplicates is the non-hostility guarantee: a
// household that already has two same-named siblings must still be able to edit
// one without renaming it.
func TestNameChangedProtectsExistingDuplicates(t *testing.T) {
	cats := []domain.Category{
		cat("a", "Groceries", "", domain.KindExpense),
		cat("b", "Groceries", "", domain.KindExpense), // pre-existing duplicate
	}
	// Editing "b" without touching its name is not a rename, so the rule sits out.
	edited := cat("b", "Groceries", "", domain.KindExpense)
	edited.Color = "#8b5cf6"
	if NameChanged(cats, edited) {
		t.Error("an edit that leaves the name alone must not count as a rename")
	}
	// Whitespace-only churn is not a rename either.
	if NameChanged(cats, cat("b", "  Groceries ", "", domain.KindExpense)) {
		t.Error("re-typing the same name with different spacing is not a rename")
	}
	// An actual rename onto a taken name IS caught.
	renamed := cat("b", "Gas", "", domain.KindExpense)
	if !NameChanged(cats, renamed) {
		t.Error("a genuine rename must count as one")
	}
	// A brand-new category always counts as named.
	if !NameChanged(cats, cat("new", "Anything", "", domain.KindExpense)) {
		t.Error("a category with no stored copy is always newly named")
	}
}

func TestSortedDoesNotMutateInput(t *testing.T) {
	in := []domain.Category{cat("1", "Zeta", "", domain.KindExpense), cat("2", "Alpha", "", domain.KindExpense)}
	out := Sorted(in)
	if in[0].Name != "Zeta" {
		t.Error("Sorted must not reorder the caller's slice")
	}
	if out[0].Name != "Alpha" || out[1].Name != "Zeta" {
		t.Errorf("Sorted = %v", out)
	}
}

// TestFindByNameSpansTheWholeTree is the counterpart to FindSibling: a household
// that already has "Auto > Auto loans" and is handed a second, top-level "Auto
// loans" experiences that as a duplicate, whatever the tree says about levels.
func TestFindByNameSpansTheWholeTree(t *testing.T) {
	cats := []domain.Category{
		cat("auto", "Auto", "", domain.KindExpense),
		cat("loans", "Auto loans", "auto", domain.KindExpense),
		cat("food", "Food", "", domain.KindExpense),
	}
	got := FindByName(cats, "auto loans")
	if len(got) != 1 || got[0].ID != "loans" {
		t.Fatalf("FindByName = %+v, want the nested Auto loans", got)
	}
	if len(FindByName(cats, "nothing")) != 0 {
		t.Error("an absent name must find nothing")
	}
	if len(FindByName(cats, "  ")) != 0 {
		t.Error("a blank name must find nothing")
	}
	// Two categories can legitimately share a name at different levels; the
	// caller decides what to do, so both are returned.
	cats = append(cats, cat("loans2", "Auto loans", "", domain.KindExpense))
	if got := FindByName(cats, "Auto Loans"); len(got) != 2 {
		t.Errorf("FindByName = %d matches, want 2", len(got))
	}
}

func TestPath(t *testing.T) {
	cats := []domain.Category{
		cat("auto", "Auto", "", domain.KindExpense),
		cat("loans", "Auto loans", "auto", domain.KindExpense),
		cat("orphan", "Orphan", "missing", domain.KindExpense),
	}
	if got := Path(cats, cats[1]); got != "Auto > Auto loans" {
		t.Errorf("Path = %q, want %q", got, "Auto > Auto loans")
	}
	if got := Path(cats, cats[0]); got != "Auto" {
		t.Errorf("Path = %q, want %q", got, "Auto")
	}
	// A dangling parent must not blow up or invent a segment.
	if got := Path(cats, cats[2]); got != "Orphan" {
		t.Errorf("Path with a missing parent = %q, want %q", got, "Orphan")
	}
	// A self-referencing parent must terminate.
	selfy := cat("selfy", "Selfy", "selfy", domain.KindExpense)
	if got := Path([]domain.Category{selfy}, selfy); got != "Selfy" {
		t.Errorf("Path with a self-parent = %q, want %q", got, "Selfy")
	}
}
