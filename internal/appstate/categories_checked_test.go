// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestCategoriesCheckedPreservesNaturalOrderAndReadFailures(t *testing.T) {
	app := newApp(t, false)
	for _, category := range []domain.Category{
		{ID: "ten", Name: "Item 10", Kind: domain.KindExpense},
		{ID: "two", Name: "Item 2", Kind: domain.KindExpense},
		{ID: "apple", Name: "apple", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := app.CategoriesChecked()
	if err != nil || len(rows) != 3 {
		t.Fatalf("categories=%v, err=%v", rows, err)
	}
	for i, want := range []string{"apple", "two", "ten"} {
		if rows[i].ID != want {
			t.Fatalf("category %d=%q, want %q", i, rows[i].ID, want)
		}
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CategoriesChecked(); err == nil {
		t.Fatal("closed store was reported as a known empty category collection")
	}
}
