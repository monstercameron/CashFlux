// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestPutCategoriesAtomicRejectsWholeBatchBeforeWriting(t *testing.T) {
	a := newApp(t, false)
	first := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}
	second := domain.Category{ID: "home", Name: "Home", Kind: domain.KindExpense}
	for _, category := range []domain.Category{first, second} {
		if err := a.PutCategory(category); err != nil {
			t.Fatalf("seed category %q: %v", category.ID, err)
		}
	}

	first.CategoryClass = domain.ClassFixed
	second.CategoryClass = "not-a-category-class"
	if err := a.PutCategoriesAtomic([]domain.Category{first, second}); err == nil {
		t.Fatal("PutCategoriesAtomic accepted an invalid category")
	}

	for _, want := range []domain.Category{
		{ID: "food", Name: "Food", Kind: domain.KindExpense},
		{ID: "home", Name: "Home", Kind: domain.KindExpense},
	} {
		got, ok, err := a.Store().GetCategory(want.ID)
		if err != nil || !ok {
			t.Fatalf("read category %q: ok=%v err=%v", want.ID, ok, err)
		}
		if got.CategoryClass != want.CategoryClass {
			t.Errorf("category %q class = %q after rejected batch, want %q", want.ID, got.CategoryClass, want.CategoryClass)
		}
	}
}

func TestPutCategoriesAtomicWritesOneUndoableCapture(t *testing.T) {
	a := newApp(t, false)
	first := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}
	second := domain.Category{ID: "home", Name: "Home", Kind: domain.KindExpense}
	for _, category := range []domain.Category{first, second} {
		if err := a.PutCategory(category); err != nil {
			t.Fatalf("seed category %q: %v", category.ID, err)
		}
	}
	start, err := a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}

	first.CategoryClass = domain.ClassFixed
	second.CategoryClass = domain.ClassNonMonthly
	if err := a.PutCategoriesAtomic([]domain.Category{first, second}); err != nil {
		t.Fatalf("PutCategoriesAtomic: %v", err)
	}
	end, err := a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	page, err := a.Store().CaptureChanges(start, end, 10)
	if err != nil {
		t.Fatalf("CaptureChanges: %v", err)
	}
	if len(page.Changes) != 2 {
		t.Fatalf("captured %d changes, want one per category: %+v", len(page.Changes), page.Changes)
	}
	for _, change := range page.Changes {
		if change.Table != "categories" {
			t.Errorf("captured table = %q, want categories", change.Table)
		}
	}

	undo := make([]store.RecordChange, len(page.Changes))
	for i, change := range page.Changes {
		undo[len(page.Changes)-1-i] = store.RecordChange{Table: change.Table, ID: change.ID, Before: change.After, After: change.Before}
	}
	if err := a.Store().ReplayChangesExpected(undo); err != nil {
		t.Fatalf("undo captured category batch: %v", err)
	}
	for _, id := range []string{first.ID, second.ID} {
		got, ok, err := a.Store().GetCategory(id)
		if err != nil || !ok {
			t.Fatalf("read category %q after undo: ok=%v err=%v", id, ok, err)
		}
		if got.CategoryClass != "" {
			t.Errorf("category %q class after undo = %q, want empty", id, got.CategoryClass)
		}
	}
}
