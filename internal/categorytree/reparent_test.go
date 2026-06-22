package categorytree

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestReparentOnDelete(t *testing.T) {
	cats := []domain.Category{
		{ID: "food", Name: "Food"},
		{ID: "groceries", Name: "Groceries", ParentID: "food"},
		{ID: "dining", Name: "Dining", ParentID: "food"},
		{ID: "coffee", Name: "Coffee", ParentID: "dining"},
		{ID: "rent", Name: "Rent"},
	}

	// Deleting a mid-level parent re-homes its children to the grandparent.
	got := ReparentOnDelete(cats, "dining")
	if len(got) != 1 || got[0].ID != "coffee" || got[0].ParentID != "food" {
		t.Fatalf("deleting dining: got %#v, want coffee re-homed to food", got)
	}

	// Deleting a top-level parent re-homes its children to root ("").
	got = ReparentOnDelete(cats, "food")
	homes := map[string]string{}
	for _, c := range got {
		homes[c.ID] = c.ParentID
	}
	if homes["groceries"] != "" || homes["dining"] != "" {
		t.Fatalf("deleting food: children should go to root, got %#v", homes)
	}
	if _, ok := homes["coffee"]; ok {
		t.Errorf("coffee is a grandchild, not a direct child — should not be returned")
	}

	// A leaf with no children yields nothing; empty id yields nothing.
	if r := ReparentOnDelete(cats, "rent"); r != nil {
		t.Errorf("deleting a childless category should return nil, got %#v", r)
	}
	if r := ReparentOnDelete(cats, ""); r != nil {
		t.Errorf("empty id should return nil, got %#v", r)
	}
}
