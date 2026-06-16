package categorytree

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func cat(id, name, parent string) domain.Category {
	return domain.Category{ID: id, Name: name, Kind: domain.KindExpense, ParentID: parent}
}

func TestBuildNestsChildren(t *testing.T) {
	cats := []domain.Category{
		cat("food", "Food", ""),
		cat("rest", "Restaurants", "food"),
		cat("groc", "Groceries", "food"),
		cat("rent", "Rent", ""),
	}
	roots := Build(cats)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
	// Roots sorted by name: Food, Rent.
	if roots[0].Category.ID != "food" || roots[1].Category.ID != "rent" {
		t.Errorf("root order = %s,%s", roots[0].Category.ID, roots[1].Category.ID)
	}
	// Food's children sorted by name: Groceries, Restaurants.
	kids := roots[0].Children
	if len(kids) != 2 || kids[0].Category.ID != "groc" || kids[1].Category.ID != "rest" {
		t.Errorf("food children = %+v", kids)
	}
}

func TestBuildOrphanIsRoot(t *testing.T) {
	cats := []domain.Category{cat("a", "A", "missing")}
	roots := Build(cats)
	if len(roots) != 1 || roots[0].Category.ID != "a" {
		t.Errorf("orphan should be a root, got %+v", roots)
	}
}

func TestBuildCycleSafe(t *testing.T) {
	// a -> b -> a : neither is a root, so both are dropped (no infinite loop).
	cats := []domain.Category{cat("a", "A", "b"), cat("b", "B", "a")}
	roots := Build(cats) // must terminate
	if len(roots) != 0 {
		t.Errorf("cyclic categories should produce no roots, got %d", len(roots))
	}
	// Self-reference is treated as a root.
	self := Build([]domain.Category{cat("s", "S", "s")})
	if len(self) != 1 || self[0].Category.ID != "s" {
		t.Errorf("self-referencing category should be a root, got %+v", self)
	}
}

func TestFlattenDepth(t *testing.T) {
	cats := []domain.Category{
		cat("food", "Food", ""),
		cat("rest", "Restaurants", "food"),
	}
	flat := Flatten(cats)
	if len(flat) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(flat))
	}
	if flat[0].Category.ID != "food" || flat[0].Depth != 0 {
		t.Errorf("entry0 = %+v", flat[0])
	}
	if flat[1].Category.ID != "rest" || flat[1].Depth != 1 {
		t.Errorf("entry1 = %+v", flat[1])
	}
}
