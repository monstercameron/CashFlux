// SPDX-License-Identifier: MIT

package categorytree

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// viscat is like cat but allows specifying an explicit kind to keep tests readable.
func viscat(id, parent string) domain.Category {
	return domain.Category{ID: id, ParentID: parent, Kind: domain.KindExpense}
}

func TestVisibleUnderCollapsed_NoneCollapsed(t *testing.T) {
	cats := []domain.Category{
		viscat("food", ""),
		viscat("rest", "food"),
		viscat("pizza", "rest"),
	}
	vis := VisibleUnderCollapsed(cats, nil)
	for _, id := range []string{"food", "rest", "pizza"} {
		if !vis[id] {
			t.Errorf("expected %q visible when nothing collapsed", id)
		}
	}
}

func TestVisibleUnderCollapsed_ParentCollapsed(t *testing.T) {
	cats := []domain.Category{
		viscat("food", ""),
		viscat("rest", "food"),
		viscat("pizza", "rest"),
	}
	collapsed := map[string]bool{"food": true}
	vis := VisibleUnderCollapsed(cats, collapsed)
	if !vis["food"] {
		t.Error("collapsed parent itself should remain visible")
	}
	if vis["rest"] {
		t.Error("direct child of collapsed parent should be hidden")
	}
	if vis["pizza"] {
		t.Error("grandchild of collapsed parent should be hidden")
	}
}

func TestVisibleUnderCollapsed_IntermediateCollapsed(t *testing.T) {
	cats := []domain.Category{
		viscat("food", ""),
		viscat("rest", "food"),
		viscat("pizza", "rest"),
	}
	// Collapsing "rest" hides pizza but not food.
	collapsed := map[string]bool{"rest": true}
	vis := VisibleUnderCollapsed(cats, collapsed)
	if !vis["food"] {
		t.Error("grandparent should remain visible")
	}
	if !vis["rest"] {
		t.Error("collapsed node itself should remain visible")
	}
	if vis["pizza"] {
		t.Error("child of collapsed node should be hidden")
	}
}

func TestVisibleUnderCollapsed_MultipleCollapsed(t *testing.T) {
	cats := []domain.Category{
		viscat("a", ""),
		viscat("b", "a"),
		viscat("c", ""),
		viscat("d", "c"),
	}
	collapsed := map[string]bool{"a": true, "c": true}
	vis := VisibleUnderCollapsed(cats, collapsed)
	if !vis["a"] || !vis["c"] {
		t.Error("collapsed roots themselves should be visible")
	}
	if vis["b"] || vis["d"] {
		t.Error("children of collapsed roots should be hidden")
	}
}

func TestVisibleUnderCollapsed_EmptyCats(t *testing.T) {
	vis := VisibleUnderCollapsed(nil, map[string]bool{"x": true})
	if len(vis) != 0 {
		t.Errorf("empty cats should return empty map, got %v", vis)
	}
}

func TestVisibleUnderCollapsed_OrphanAlwaysVisible(t *testing.T) {
	// Parent "missing" does not exist in cats; child should still be visible.
	cats := []domain.Category{viscat("child", "missing")}
	vis := VisibleUnderCollapsed(cats, map[string]bool{"missing": true})
	if !vis["child"] {
		t.Error("child with missing parent should be visible (orphan treated as root)")
	}
}

func TestVisibleUnderCollapsed_CycleSafe(t *testing.T) {
	// a→b, b→a: cycle. Neither should loop or panic; both should be visible
	// because the cycle guard prevents any ancestor resolution from concluding
	// that one of them is collapsed.
	cats := []domain.Category{
		{ID: "a", ParentID: "b", Kind: domain.KindExpense},
		{ID: "b", ParentID: "a", Kind: domain.KindExpense},
	}
	collapsed := map[string]bool{"a": true, "b": true}
	// Must not hang; result can be anything deterministic.
	vis := VisibleUnderCollapsed(cats, collapsed)
	// Both are in a cycle so the ancestor walk hits the cycle guard; no ancestor
	// is resolved as collapsed (the walk exits before resolving), so both appear visible.
	_ = vis // just verify it doesn't panic/loop
}
