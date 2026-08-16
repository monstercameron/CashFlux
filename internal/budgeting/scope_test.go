// SPDX-License-Identifier: MIT

package budgeting

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestScopeOfIncludesDescendants(t *testing.T) {
	b := domain.Budget{ID: "b1", CategoryID: "transport"}
	desc := map[string]bool{"transport": true, "gas": true, "autoloan": true}

	got := ScopeOf(b, desc)
	want := []string{"autoloan", "gas", "transport"}
	if !reflect.DeepEqual(got.CategoryIDs, want) {
		t.Errorf("CategoryIDs = %v, want %v", got.CategoryIDs, want)
	}
	if !got.HasDescendants(b) {
		t.Error("HasDescendants = false, want true (gas and autoloan are past the tracked set)")
	}
}

func TestScopeOfMultiCategoryAndTags(t *testing.T) {
	b := domain.Budget{
		ID: "b1", CategoryID: "travel",
		CategoryIDs: []string{"travel", "dining"},
		TrackedTags: []string{"vacation"},
	}
	got := ScopeOf(b, map[string]bool{"travel": true, "dining": true})

	if want := []string{"dining", "travel"}; !reflect.DeepEqual(got.CategoryIDs, want) {
		t.Errorf("CategoryIDs = %v, want %v", got.CategoryIDs, want)
	}
	if want := []string{"vacation"}; !reflect.DeepEqual(got.Tags, want) {
		t.Errorf("Tags = %v, want %v", got.Tags, want)
	}
	if got.HasDescendants(b) {
		t.Error("HasDescendants = true, want false — the scope is exactly the tracked categories")
	}
	if got.IsEmpty() {
		t.Error("IsEmpty = true for a scope with categories and a tag")
	}
}

func TestScopeOfEmptyBudget(t *testing.T) {
	got := ScopeOf(domain.Budget{ID: "b1"}, nil)
	if !got.IsEmpty() {
		t.Errorf("IsEmpty = false for %+v, want true", got)
	}
}

// The scope must be the same set the bar counts against: whatever ScopeOf
// returns, TracksCategory-or-descendant must accept, and vice versa.
func TestScopeOfMatchesTheRollupCover(t *testing.T) {
	b := domain.Budget{ID: "b1", CategoryID: "transport"}
	desc := map[string]bool{"gas": true}
	covers := func(id string) bool { return b.TracksCategory(id) || desc[id] }

	scope := ScopeOf(b, desc)
	for _, id := range scope.CategoryIDs {
		if !covers(id) {
			t.Errorf("scope includes %q but the rollup cover rejects it", id)
		}
	}
	for _, id := range []string{"transport", "gas"} {
		found := false
		for _, got := range scope.CategoryIDs {
			if got == id {
				found = true
			}
		}
		if !found {
			t.Errorf("the rollup counts %q but the scope omits it", id)
		}
	}
}
