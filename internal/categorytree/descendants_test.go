// SPDX-License-Identifier: MIT

package categorytree

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// food → groceries → organic; food → restaurants; transport (separate root).
func rollupCats() []domain.Category {
	return []domain.Category{
		cat("food", "Food", ""),
		cat("groc", "Groceries", "food"),
		cat("org", "Organic", "groc"),
		cat("rest", "Restaurants", "food"),
		cat("transport", "Transport", ""),
	}
}

func TestDescendantsMultiLevel(t *testing.T) {
	got := Descendants(rollupCats(), "food")
	want := map[string]bool{"food": true, "groc": true, "org": true, "rest": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Descendants(food) = %v, want %v", got, want)
	}
	// A mid-tree node rolls up only its own subtree.
	if got := Descendants(rollupCats(), "groc"); !reflect.DeepEqual(got, map[string]bool{"groc": true, "org": true}) {
		t.Errorf("Descendants(groc) = %v, want {groc, org}", got)
	}
	// A leaf is just itself.
	if got := Descendants(rollupCats(), "org"); !reflect.DeepEqual(got, map[string]bool{"org": true}) {
		t.Errorf("Descendants(org) = %v, want {org}", got)
	}
}

func TestDescendantsReparent(t *testing.T) {
	cats := rollupCats()
	// Reparent Groceries (and its child Organic) from Food to Transport.
	for i := range cats {
		if cats[i].ID == "groc" {
			cats[i].ParentID = "transport"
		}
	}
	if got := Descendants(cats, "transport"); !reflect.DeepEqual(got, map[string]bool{"transport": true, "groc": true, "org": true}) {
		t.Errorf("after reparent, Descendants(transport) = %v, want {transport, groc, org}", got)
	}
	if got := Descendants(cats, "food"); !reflect.DeepEqual(got, map[string]bool{"food": true, "rest": true}) {
		t.Errorf("after reparent, Descendants(food) = %v, want {food, rest}", got)
	}
}

func TestDescendantsEdgeCases(t *testing.T) {
	if got := Descendants(rollupCats(), ""); len(got) != 0 {
		t.Errorf("empty rootID = %v, want empty", got)
	}
	if got := Descendants(rollupCats(), "unknown"); !reflect.DeepEqual(got, map[string]bool{"unknown": true}) {
		t.Errorf("unknown rootID = %v, want {unknown}", got)
	}
	// A self-referential / cyclic parent must not loop.
	cyc := []domain.Category{cat("a", "A", "b"), cat("b", "B", "a")}
	got := Descendants(cyc, "a")
	if !got["a"] || !got["b"] {
		t.Errorf("cycle Descendants(a) = %v, want to include a and b without looping", got)
	}
}

func TestDescendantsOfAllMatchesPerRootUnionAndDoesNotMutate(t *testing.T) {
	cats := []domain.Category{
		cat("a", "A", "b"), // cycle with b
		cat("b", "B", "a"),
		cat("self", "Self", "self"), // self-parent is not an adjacency edge
		cat("orphan", "Orphan", "outside"),
		cat("child", "Child", "orphan"),
		cat("plain", "Plain", ""),
	}
	before := append([]domain.Category(nil), cats...)
	roots := []string{"a", "b", "a", "", "self", "outside", "unknown"}
	want := map[string]bool{
		"a": true, "b": true, "self": true, "outside": true,
		"orphan": true, "child": true, "unknown": true,
	}
	got := DescendantsOfAll(cats, roots)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DescendantsOfAll edge union = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(cats, before) {
		t.Fatalf("DescendantsOfAll mutated categories: got %v, want %v", cats, before)
	}

	// Keep the public union contract pinned to the established per-root traversal.
	parity := make(map[string]bool)
	for _, root := range roots {
		for id := range Descendants(cats, root) {
			parity[id] = true
		}
	}
	if !reflect.DeepEqual(got, parity) {
		t.Fatalf("DescendantsOfAll differs from union of Descendants calls: got %v, want %v", got, parity)
	}
}

func descendantsOfAllBenchmarkFixture() ([]domain.Category, []string) {
	const categoryCount = 2047
	cats := make([]domain.Category, categoryCount)
	roots := make([]string, 0, categoryCount/4)
	for i := range cats {
		id := fmt.Sprintf("cat-%04d", i)
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("cat-%04d", (i-1)/2)
		}
		cats[i] = domain.Category{ID: id, ParentID: parent, Name: id}
		// The selected roots overlap heavily in the balanced tree: selected
		// ancestors already cover many of the descendants of later roots.
		if i%4 == 0 {
			roots = append(roots, id)
		}
	}
	return cats, roots
}

func BenchmarkDescendantsOfAllManyRoots(b *testing.B) {
	cats, roots := descendantsOfAllBenchmarkFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DescendantsOfAll(cats, roots)
	}
}
