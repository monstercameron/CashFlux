// SPDX-License-Identifier: MIT

package validate

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// C495: categorytree.Build only re-roots a bad parent at RENDER time, so without
// these rules a self-parented or looping category persists happily in the store
// and merely looks odd on screen.
func TestValidateCategoryRejectsSelfParent(t *testing.T) {
	c := domain.Category{ID: "c1", Name: "Auto", Kind: domain.KindExpense, ParentID: "c1"}
	if is := ValidateCategory(c); is.OK() {
		t.Error("a category that is its own parent must be rejected")
	}
	c.ParentID = ""
	if is := ValidateCategory(c); !is.OK() {
		t.Errorf("a valid top-level category was rejected: %v", is)
	}
}

func TestValidateCategoryInTree(t *testing.T) {
	existing := []domain.Category{
		{ID: "auto", Name: "Auto", Kind: domain.KindExpense},
		{ID: "gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "auto"},
		{ID: "salary", Name: "Salary", Kind: domain.KindIncome},
	}

	t.Run("valid child", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "Tolls", Kind: domain.KindExpense, ParentID: "auto"}
		if is := ValidateCategoryInTree(c, existing); !is.OK() {
			t.Errorf("a valid child was rejected: %v", is)
		}
	})

	t.Run("top level always fine", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "Pets", Kind: domain.KindExpense}
		if is := ValidateCategoryInTree(c, existing); !is.OK() {
			t.Errorf("a top-level category was rejected: %v", is)
		}
	})

	t.Run("missing parent", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "Ghost", Kind: domain.KindExpense, ParentID: "nope"}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Error("a parent that does not exist must be rejected")
		}
	})

	t.Run("kind mismatch", func(t *testing.T) {
		// An expense nested under income would roll up into a meaningless total.
		c := domain.Category{ID: "new", Name: "Bonus spend", Kind: domain.KindExpense, ParentID: "salary"}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Error("a child whose kind differs from its parent must be rejected")
		}
	})

	t.Run("direct loop", func(t *testing.T) {
		// Auto is Gas's parent; making Gas the parent of Auto closes the loop.
		c := domain.Category{ID: "auto", Name: "Auto", Kind: domain.KindExpense, ParentID: "gas"}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Error("an edge that closes a loop must be rejected")
		}
	})

	t.Run("longer loop", func(t *testing.T) {
		deep := []domain.Category{
			{ID: "a", Name: "A", Kind: domain.KindExpense},
			{ID: "b", Name: "B", Kind: domain.KindExpense, ParentID: "a"},
			{ID: "c", Name: "C", Kind: domain.KindExpense, ParentID: "b"},
		}
		c := domain.Category{ID: "a", Name: "A", Kind: domain.KindExpense, ParentID: "c"}
		if is := ValidateCategoryInTree(c, deep); is.OK() {
			t.Error("a three-node loop must be rejected")
		}
	})

	t.Run("pre-existing cycle does not hang", func(t *testing.T) {
		// Corrupt stored data: x and y already point at each other. The walk is
		// bounded, so validation must return rather than spin.
		broken := []domain.Category{
			{ID: "x", Name: "X", Kind: domain.KindExpense, ParentID: "y"},
			{ID: "y", Name: "Y", Kind: domain.KindExpense, ParentID: "x"},
		}
		c := domain.Category{ID: "new", Name: "New", Kind: domain.KindExpense, ParentID: "x"}
		done := make(chan Issues, 1)
		go func() { done <- ValidateCategoryInTree(c, broken) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ValidateCategoryInTree hung on a pre-existing cycle")
		}
	})
}

// timeoutGuard keeps the cycle test honest without a sleep in the happy path.
var _ = time.Second

// TestValidateCategoryNameUniqueAmongSiblings covers C537: the write seam is the
// one place a duplicate name can be stopped, because eight screens create
// categories and only one of them ever checked.
func TestValidateCategoryNameUniqueAmongSiblings(t *testing.T) {
	existing := []domain.Category{
		{ID: "auto", Name: "Auto", Kind: domain.KindExpense},
		{ID: "gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "auto"},
		{ID: "food", Name: "Food", Kind: domain.KindExpense},
		{ID: "salary", Name: "Salary", Kind: domain.KindIncome},
	}

	t.Run("a new top-level duplicate is rejected", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "food", Kind: domain.KindExpense}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Fatal("a second top-level Food must be rejected")
		}
	})

	t.Run("a duplicate under the same parent is rejected", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: " GAS ", Kind: domain.KindExpense, ParentID: "auto"}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Fatal("a second Gas under Auto must be rejected regardless of case or spacing")
		}
	})

	t.Run("the same name under a different parent is allowed", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "Gas", Kind: domain.KindExpense}
		if is := ValidateCategoryInTree(c, existing); !is.OK() {
			t.Fatalf("a top-level Gas must be allowed alongside Auto > Gas: %v", is)
		}
	})

	t.Run("kind does not excuse a collision", func(t *testing.T) {
		// An income "Food" beside an expense "Food" is indistinguishable in every
		// list the app draws, which is exactly the reported complaint.
		c := domain.Category{ID: "new", Name: "Food", Kind: domain.KindIncome}
		if is := ValidateCategoryInTree(c, existing); is.OK() {
			t.Fatal("a same-named income category must still collide")
		}
	})

	t.Run("re-saving a category does not collide with itself", func(t *testing.T) {
		c := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense, Color: "#8b5cf6"}
		if is := ValidateCategoryInTree(c, existing); !is.OK() {
			t.Fatalf("editing a category must not trip over its own name: %v", is)
		}
	})

	t.Run("pre-existing duplicates stay editable", func(t *testing.T) {
		// The non-hostility guarantee: data that already contains a duplicate pair
		// must not have both members frozen against every future edit.
		dupes := append(append([]domain.Category(nil), existing...),
			domain.Category{ID: "food2", Name: "Food", Kind: domain.KindExpense})
		c := domain.Category{ID: "food2", Name: "Food", Kind: domain.KindExpense, Color: "#111"}
		if is := ValidateCategoryInTree(c, dupes); !is.OK() {
			t.Fatalf("an unrelated edit to an existing duplicate must be allowed: %v", is)
		}
		// But renaming it ONTO another taken name is still caught.
		c2 := domain.Category{ID: "food2", Name: "Auto", Kind: domain.KindExpense}
		if is := ValidateCategoryInTree(c2, dupes); is.OK() {
			t.Fatal("renaming onto a taken sibling name must be rejected")
		}
	})

	t.Run("the message names the category that already holds it", func(t *testing.T) {
		c := domain.Category{ID: "new", Name: "food", Kind: domain.KindExpense}
		is := ValidateCategoryInTree(c, existing)
		if is.OK() || !strings.Contains(is.Error(), "Food") {
			t.Fatalf("the error must name the existing category, got %v", is)
		}
	})
}
