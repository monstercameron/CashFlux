// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestTV8ETransactionCopy(t *testing.T) {
	bundle := DefaultBundle()
	cases := []struct {
		name string
		key  string
		args []any
		want string
	}{
		{"row category toast", "r10w.categoryFiled", []any{"Groceries"}, "Filed under Groceries"},
		{"bulk category toast", "transactions.bulkCatApplied", []any{10, "Gifts & Charity"}, "10 updated; filed under Gifts & Charity"},
		{"undo story", "toast.undoStory", []any{"Filed under Groceries"}, "Filed under Groceries"},
		{"split category count", "splitEditor.cellLabel", []any{2}, "2 categories"},
		{"missing transfer account", "transactions.scopeUnpaired", []any{3}, "3 transfers are missing their other account"},
		{"one bulk category update", "transactions.bulkCatAppliedOne", []any{"Groceries"}, "Filed under Groceries for 1 transaction"},
		{"many bulk category updates", "transactions.bulkCatAppliedOther", []any{"Groceries", 3}, "Filed under Groceries for 3 transactions"},
		{"one filter clear", "transactions.clearAllFiltersOne", nil, "Clear filter"},
		{"many filters clear", "transactions.clearAllFiltersOther", []any{3}, "Clear 3 filters"},
		{"expense menu", "addmenu.expense", nil, "Expense"},
		{"undo failure", "bv6d1.undoSaveFailed", nil, "Couldn't undo. Your change is still in place. Retry"},
		{"activity undo failure", "bv6d1.undoTheSaveFailed", nil, "Couldn't undo. Your change is still in place. Retry"},
		{"unusual charge title", "smart.u1.title", []any{"Unusually large", "$120.00"}, "Unusually large charge: $120.00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bundle.T(English, tc.key, tc.args...); got != tc.want {
				t.Errorf("T(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestTV8EBulkCategoryLegacyCopyHasNoPluralNoun(t *testing.T) {
	bundle := DefaultBundle()
	got := bundle.T(English, "transactions.bulkCatApplied", 1, "Gifts & Charity")
	if got != "1 updated; filed under Gifts & Charity" {
		t.Fatalf("single-item bulk copy = %q", got)
	}
}
