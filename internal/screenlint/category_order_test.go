// SPDX-License-Identifier: MIT

package screenlint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── C518/C544 category-ordering ratchet ─────────────────────────────────────
//
// Category lists are ordered ONCE, at the seam every reader passes through:
// appstate.App.Categories() returns catname.Sorted(store.ListCategories()).
// That is what makes a new picker correct by default instead of correct only if
// its author remembered to sort — the failure that let C518 close green while
// every flat picker in the app still rendered store order.
//
// Two ways to reintroduce it, both guarded here:
//
//  1. Bypass the seam by calling the store's ListCategories directly. Exactly
//     one caller outside internal/store is allowed: appstate.Categories, which
//     IS the seam.
//  2. Re-sort a category slice with a raw byte comparison, which silently undoes
//     the natural ordering — "Zoo" before "apple", "Item 10" before "Item 9".
//     catname.Less is the one comparison in the app.

// goFilesUnder returns every non-test .go file beneath root, relative to it.
func goFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	return out
}

// internalRoot locates internal/ from this package's directory.
func internalRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..")
}

// TestListCategoriesHasOneCallerOutsideTheStore keeps every reader on the sorted
// seam. A new direct caller gets store (insertion) order and quietly reproduces
// the "my categories are missing" report.
func TestListCategoriesHasOneCallerOutsideTheStore(t *testing.T) {
	root := internalRoot(t)
	var offenders []string
	for _, path := range goFilesUnder(t, root) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("rel %q: %v", path, err)
		}
		rel = filepath.ToSlash(rel)
		// The store defines it; appstate is the sanctioned single caller.
		if strings.HasPrefix(rel, "store/") || rel == "appstate/appstate.go" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		if strings.Contains(string(data), "ListCategories()") {
			offenders = append(offenders, rel)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("ListCategories() called outside the ordering seam in %v.\n"+
			"Use appstate.App.Categories(), which returns catname.Sorted(...). Calling the store "+
			"directly yields insertion order, which reads as a broken/incomplete list (C518/C544).",
			offenders)
	}
}

// TestNoRawByteSortOnCategoryNames catches a re-sort that undoes the natural
// ordering. It looks for the exact shape a byte comparison takes in a sort
// closure over a category slice.
func TestNoRawByteSortOnCategoryNames(t *testing.T) {
	root := internalRoot(t)
	// Identifiers the codebase uses for a []domain.Category, paired with the
	// byte-compare shape `x[i].Name < x[j].Name`.
	catVars := []string{"cats", "categories", "shown", "parents", "expenseCats", "pickableCats"}
	var offenders []string
	for _, path := range goFilesUnder(t, root) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		text := string(data)
		for _, v := range catVars {
			needle := v + "[i].Name < " + v + "[j].Name"
			if strings.Contains(text, needle) {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, filepath.ToSlash(rel)+": "+needle)
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("raw byte comparison sorting category names in %v.\n"+
			"Use catname.Less (or catname.Sorted): a byte compare is case-sensitive "+
			"(\"Zoo\" before \"apple\") and non-natural (\"Item 10\" before \"Item 9\"), so the list "+
			"disagrees with every other category list in the app (C544).",
			offenders)
	}
}
