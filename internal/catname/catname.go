// SPDX-License-Identifier: MIT

// Package catname is the single authority on how category names are compared:
// how two names are judged equal, how a list of them is ordered, and how a typed
// name resolves to a category that already exists.
//
// It exists because those three questions used to be answered independently in
// every screen that asked them, and the answers disagreed. The add-a-budget form
// matched names case-insensitively across the whole tree but only against expense
// categories; the category tree sorted siblings with a plain string compare, so
// "Item 10" sorted before "Item 9"; and nothing at the write seam checked for a
// collision at all, so six other screens could mint a duplicate in silence.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package catname

import (
	"sort"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Normalize reduces a category name to its comparison form: trimmed, folded to
// lower case, and with every run of internal whitespace collapsed to one space.
//
// The whitespace collapse matters more than it looks: "Eating  Out" typed with a
// double space is the same category to a human, and treating it as a new one is
// exactly the silent-duplicate behavior this package exists to stop.
func Normalize(name string) string {
	return strings.Join(strings.FieldsFunc(name, unicode.IsSpace), " ")
}

// EqualNames reports whether two category names refer to the same name.
func EqualNames(a, b string) bool {
	na, nb := Normalize(a), Normalize(b)
	return na != "" && strings.EqualFold(na, nb)
}

// Less orders two category names the way a person reading a list expects, which
// is NOT plain lexicographic order: digit runs compare as numbers, so "Item 9"
// comes before "Item 10" rather than after it. Comparison is case-insensitive,
// with a case-sensitive tiebreak so the order is total and stable.
func Less(a, b string) bool {
	na, nb := strings.ToLower(Normalize(a)), strings.ToLower(Normalize(b))
	if c := compareNatural(na, nb); c != 0 {
		return c < 0
	}
	return a < b
}

// compareNatural returns -1, 0 or 1 comparing a and b with digit runs treated as
// numbers. Both inputs are expected to be already normalized and folded.
func compareNatural(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		ad, bd := unicode.IsDigit(ar[i]), unicode.IsDigit(br[j])
		if ad && bd {
			// Compare the whole digit runs as numbers. Leading zeros are skipped
			// for the magnitude test, then used only to break an exact tie, so
			// "007" and "7" order next to each other rather than far apart.
			si, sj := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}
			x := strings.TrimLeft(string(ar[si:i]), "0")
			y := strings.TrimLeft(string(br[sj:j]), "0")
			if len(x) != len(y) {
				// A longer digit string (zeros stripped) is the larger number —
				// no big-integer parse needed, which keeps arbitrarily long runs
				// from overflowing.
				return sign(len(x) - len(y))
			}
			if x != y {
				return sign(strings.Compare(x, y))
			}
			if c := (j - sj) - (i - si); c != 0 {
				return sign(c) // equal value, more leading zeros sorts first
			}
			continue
		}
		if ar[i] != br[j] {
			return sign(strings.Compare(string(ar[i]), string(br[j])))
		}
		i++
		j++
	}
	return sign((len(ar) - i) - (len(br) - j))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// Sort orders categories by name, in place, using Less. Callers that render a
// FLAT list of categories should route through here; callers that render the
// tree get the same order from categorytree.Build, which sorts siblings with the
// same comparison.
func Sort(cats []domain.Category) {
	sort.SliceStable(cats, func(i, j int) bool { return Less(cats[i].Name, cats[j].Name) })
}

// Sorted returns a name-ordered COPY of cats, for callers that must not disturb
// the store's slice.
func Sorted(cats []domain.Category) []domain.Category {
	out := make([]domain.Category, len(cats))
	copy(out, cats)
	Sort(out)
	return out
}

// FindSibling returns the category with the given name directly under parentID,
// or false when there is none.
//
// Parent-aware on purpose. Resolving a typed name against the WHOLE tree — as
// the add-a-budget form used to — means a "Gas" nested under "Auto" satisfies a
// request for a top-level "Gas", and the caller silently attaches to whichever
// one the store happens to return first. Names are only meaningful within their
// level, which is also how the tree is displayed.
//
// Kind is deliberately NOT part of the match: an income category named
// "Investments" collides with an expense category named "Investments" as far as
// a person reading the list is concerned, and minting the second one is the bug.
func FindSibling(cats []domain.Category, parentID, name string) (domain.Category, bool) {
	if Normalize(name) == "" {
		return domain.Category{}, false
	}
	for _, c := range cats {
		if c.ParentID == parentID && EqualNames(c.Name, name) {
			return c, true
		}
	}
	return domain.Category{}, false
}

// Collision returns the OTHER category that already holds c's name among c's
// siblings, or false when the name is free.
//
// Used at the write seam. It excludes c itself by ID, so re-saving an unchanged
// category never trips over its own name.
func Collision(cats []domain.Category, c domain.Category) (domain.Category, bool) {
	if Normalize(c.Name) == "" {
		return domain.Category{}, false
	}
	for _, other := range cats {
		if other.ID == c.ID || other.ParentID != c.ParentID {
			continue
		}
		if EqualNames(other.Name, c.Name) {
			return other, true
		}
	}
	return domain.Category{}, false
}

// NameChanged reports whether c's name differs from the stored copy of c, and is
// true when c is new (no stored copy).
//
// This is what keeps the uniqueness rule from being hostile to data that already
// has duplicates. A household that somehow ended up with two "Groceries" under
// the same parent must still be able to edit one's icon or class; only an edit
// that actually sets or changes the NAME is held to the rule. The remedy for the
// existing pair is a merge, not a blocked save on an unrelated field.
func NameChanged(cats []domain.Category, c domain.Category) bool {
	for _, prior := range cats {
		if prior.ID == c.ID {
			return !EqualNames(prior.Name, c.Name)
		}
	}
	return true
}

// FindByName returns every category holding the given name, ANYWHERE in the
// tree, in the order they appear.
//
// FindSibling answers "is this name taken at this level?", which is the right
// question for validation. This answers a different one the UI needs: "does a
// category by this name already exist at all?" — because a household that has
// "Auto > Auto loans" and is about to be handed a second, top-level "Auto loans"
// experiences that as a duplicate, whatever the tree says.
func FindByName(cats []domain.Category, name string) []domain.Category {
	if Normalize(name) == "" {
		return nil
	}
	var out []domain.Category
	for _, c := range cats {
		if EqualNames(c.Name, name) {
			out = append(out, c)
		}
	}
	return out
}

// Path renders a category's full display path ("Auto > Auto loans"), so a
// message about reusing a nested category can say WHICH one. Depth is bounded by
// the category count, so a cycle in stored data cannot hang the walk.
func Path(cats []domain.Category, c domain.Category) string {
	byID := make(map[string]domain.Category, len(cats))
	for _, x := range cats {
		byID[x.ID] = x
	}
	// Normalize each segment for DISPLAY (C619): a name stored with stray or
	// doubled whitespace is the same category to this package (EqualNames folds
	// it), so rendering "Work >  travel " while treating it as "travel" would let
	// the storage artefact leak onto the screen.
	parts := []string{Normalize(c.Name)}
	seen := map[string]bool{c.ID: true}
	for id, steps := c.ParentID, 0; id != "" && steps <= len(cats); steps++ {
		if seen[id] {
			break
		}
		seen[id] = true
		p, ok := byID[id]
		if !ok {
			break
		}
		parts = append([]string{Normalize(p.Name)}, parts...)
		id = p.ParentID
	}
	return strings.Join(parts, " > ")
}
