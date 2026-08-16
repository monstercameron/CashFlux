// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// seedUnsortedCategories writes categories in an order chosen to fail every
// naive comparison: reverse alphabetical, mixed case, and numeric suffixes that
// a byte compare gets wrong.
func seedUnsortedCategories(t *testing.T, app *App) {
	t.Helper()
	for _, c := range []domain.Category{
		{ID: "c-zoo", Name: "Zoo", Kind: domain.KindExpense},
		{ID: "c-item10", Name: "Item 10", Kind: domain.KindExpense},
		{ID: "c-apple", Name: "apple", Kind: domain.KindExpense},
		{ID: "c-item9", Name: "Item 9", Kind: domain.KindExpense},
		{ID: "c-bread", Name: "Bread", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(c); err != nil {
			t.Fatalf("PutCategory(%q): %v", c.Name, err)
		}
	}
}

// TestCategoriesAreNaturallySorted is the C518/C544 regression guard. Every
// category picker in the app draws from App.Categories(), so ordering it here
// orders all of them — and this test is what stops the ordering being quietly
// removed again.
func TestCategoriesAreNaturallySorted(t *testing.T) {
	app := newApp(t, false)
	seedUnsortedCategories(t, app)

	got := app.Categories()
	want := []string{"apple", "Bread", "Item 9", "Item 10", "Zoo"}
	if len(got) != len(want) {
		t.Fatalf("Categories() returned %d categories, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w {
			names := make([]string, len(got))
			for j, c := range got {
				names[j] = c.Name
			}
			t.Fatalf("Categories()[%d] = %q, want %q (full order: %v)", i, got[i].Name, w, names)
		}
	}
}

// TestCategoriesOrderIsCaseInsensitive pins the half a raw byte comparison gets
// wrong: uppercase sorts before every lowercase letter in ASCII, so "Zoo" would
// lead and "apple" would trail.
func TestCategoriesOrderIsCaseInsensitive(t *testing.T) {
	app := newApp(t, false)
	seedUnsortedCategories(t, app)

	got := app.Categories()
	if got[0].Name != "apple" {
		t.Errorf("Categories()[0] = %q, want %q — a case-sensitive byte compare has crept back in", got[0].Name, "apple")
	}
	if got[len(got)-1].Name != "Zoo" {
		t.Errorf("Categories()[last] = %q, want %q", got[len(got)-1].Name, "Zoo")
	}
}

// TestCategoriesOrderIsNatural pins the other half: "Item 10" must follow
// "Item 9", which lexicographic ordering gets backwards.
func TestCategoriesOrderIsNatural(t *testing.T) {
	app := newApp(t, false)
	seedUnsortedCategories(t, app)

	var nine, ten int = -1, -1
	for i, c := range app.Categories() {
		switch c.Name {
		case "Item 9":
			nine = i
		case "Item 10":
			ten = i
		}
	}
	if nine < 0 || ten < 0 {
		t.Fatalf("seeded categories missing from Categories(): nine=%d ten=%d", nine, ten)
	}
	if nine > ten {
		t.Errorf("Item 9 sorted at %d, after Item 10 at %d — the comparison is lexicographic, not natural", nine, ten)
	}
}

// TestCategoriesOrderSurvivesAMutation proves the per-revision read cache does
// not hand back a stale unsorted slice: a category added after the first read
// must appear in its sorted position, not appended at the end.
func TestCategoriesOrderSurvivesAMutation(t *testing.T) {
	app := newApp(t, false)
	seedUnsortedCategories(t, app)
	_ = app.Categories() // prime the cache

	if err := app.PutCategory(domain.Category{ID: "c-aaa", Name: "Aardvark", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	got := app.Categories()
	if got[0].Name != "Aardvark" {
		names := make([]string, len(got))
		for j, c := range got {
			names[j] = c.Name
		}
		t.Fatalf("after adding Aardvark, Categories()[0] = %q, want %q (full order: %v)", got[0].Name, "Aardvark", names)
	}
}
