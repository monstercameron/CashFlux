package categorytree

import (
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
