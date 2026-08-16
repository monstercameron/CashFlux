// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The tracked-categories picker is the screen behind Cam's "What this budget
// tracks might not show all categories" report. Three separate things made a
// complete list read as incomplete, and these mount the real component to prove
// each is fixed:
//
//	C544 — rows arrived in store (insertion) order, so scanning for a category
//	       felt like it was missing.
//	C550 — rows were labelled by bare name, so "Auto > Gas" and a top-level
//	       "Gas" were two identical rows.
//	C519 — non-expense categories are withheld from a spending budget, which is
//	       correct but must be stated (already shipped; pinned here so the
//	       C544/C550 changes can't silently drop the note).

// pickerTestApp installs an appstate seeded with a deliberately awkward category
// set: reverse-alphabetical insertion, mixed case, a numeric pair a byte compare
// gets backwards, a duplicated leaf name under two parents, and one income
// category the spending picker must withhold.
func pickerTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	for _, c := range []domain.Category{
		{ID: "c-zoo", Name: "Zoo", Kind: domain.KindExpense},
		{ID: "c-auto", Name: "Auto", Kind: domain.KindExpense},
		{ID: "c-home", Name: "Home", Kind: domain.KindExpense},
		{ID: "c-item10", Name: "Item 10", Kind: domain.KindExpense},
		{ID: "c-item9", Name: "Item 9", Kind: domain.KindExpense},
		{ID: "c-apple", Name: "apple", Kind: domain.KindExpense},
		// The duplicate-leaf case: same name, two different parents.
		{ID: "c-auto-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-auto"},
		{ID: "c-home-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-home"},
		// Withheld from a spending budget (C519).
		{ID: "c-salary", Name: "Salary", Kind: domain.KindIncome},
	} {
		if err := app.PutCategory(c); err != nil {
			t.Fatalf("PutCategory(%q): %v", c.Name, err)
		}
	}
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

// pickerRowLabels returns the visible label of each rendered picker row, in
// render order.
func pickerRowLabels(f *render.Fixture) []string {
	var out []string
	for _, n := range f.AllByTag("span") {
		if strings.Contains(n.Attr("class"), "budgetcat-name") {
			out = append(out, n.Text())
		}
	}
	return out
}

// TestBudgetCategoryPickerOrdersNaturally is the C544 guard at the component
// level: the picker draws app.Categories(), which is sorted at the seam, so the
// rows must come out in natural order without the picker sorting anything.
func TestBudgetCategoryPickerOrdersNaturally(t *testing.T) {
	pickerTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{Picked: map[string]bool{}}))

	got := pickerRowLabels(f)
	if len(got) == 0 {
		t.Fatalf("picker rendered no rows")
	}
	if got[0] != "apple" {
		t.Errorf("first row = %q, want %q — a case-sensitive order has crept back in (got %v)", got[0], "apple", got)
	}
	// Natural ordering: Item 9 before Item 10.
	var nine, ten = -1, -1
	for i, label := range got {
		switch label {
		case "Item 9":
			nine = i
		case "Item 10":
			ten = i
		}
	}
	if nine < 0 || ten < 0 {
		t.Fatalf("Item 9 / Item 10 missing from the picker rows: %v", got)
	}
	if nine > ten {
		t.Errorf("Item 9 rendered after Item 10 (%d > %d) — lexicographic, not natural (got %v)", nine, ten, got)
	}
}

// TestBudgetCategoryPickerLabelsRowsByPath is the C550 guard: two categories
// named "Gas" under different parents must be distinguishable, or the list reads
// as a duplicate and as missing the one you actually wanted.
func TestBudgetCategoryPickerLabelsRowsByPath(t *testing.T) {
	pickerTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{Picked: map[string]bool{}}))

	got := pickerRowLabels(f)
	var bare int
	for _, label := range got {
		if label == "Gas" {
			bare++
		}
	}
	if bare > 0 {
		t.Errorf("%d row(s) labelled bare %q — sub-categories must render their parent path (got %v)", bare, "Gas", got)
	}
	wantPaths := []string{"Auto > Gas", "Home > Gas"}
	for _, want := range wantPaths {
		found := false
		for _, label := range got {
			if label == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no row labelled %q — expected the full path (got %v)", want, got)
		}
	}
}

// TestBudgetCategoryPickerWithholdsIncomeAndSaysSo pins C519 so the C544/C550
// rework cannot silently drop the explanation of what was filtered out.
func TestBudgetCategoryPickerWithholdsIncomeAndSaysSo(t *testing.T) {
	pickerTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{Picked: map[string]bool{}}))

	for _, label := range pickerRowLabels(f) {
		if label == "Salary" {
			t.Errorf("a spending budget offered the income category %q", label)
		}
	}
	note := f.Container().Text()
	if !strings.Contains(note, "income categories are not shown") &&
		!strings.Contains(note, "income category is not shown") {
		t.Errorf("the picker withheld an income category without saying so; rendered text: %q", note)
	}
}

// TestBudgetCategoryPickerSavingOffersEverything is the other half of C519: a
// SAVING budget is the case that needs income categories, so it offers them and
// the withheld-count note stands down.
func TestBudgetCategoryPickerSavingOffersEverything(t *testing.T) {
	pickerTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{Picked: map[string]bool{}, Saving: true}))

	found := false
	for _, label := range pickerRowLabels(f) {
		if label == "Salary" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("a saving budget did not offer the income category %q (got %v)", "Salary", pickerRowLabels(f))
	}
}
