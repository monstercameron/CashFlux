// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestTransactionsCopyRoundFour(t *testing.T) {
	b := DefaultBundle()
	cases := []struct {
		key  string
		args []any
		want string
	}{
		{"addmenu.transaction", nil, "Add transaction"},
		{"quickAdd.added", nil, "Transaction added"},
		{"transactions.bulkDeleteConfirmOne", []any{1}, "Delete 1 transaction? You can undo this."},
		{"transactions.bulkDeleteConfirmMany", []any{2}, "Delete 2 transactions? You can undo this."},
		{"transactions.filterSource", nil, "Source"},
		{"reviewInbox.title", nil, "Review"},
		{"review.button", []any{247}, "Review 247"},
		{"review.tierLook", nil, "Suggestions to check"},
		{"r9c.weekSpent", []any{"−$100.00"}, "Spent −$100.00"},
		{"r9c.weekIncome", []any{"$100.00"}, "Income $100.00"},
		{"transactions.autoWhySource", nil, "Added from your bill schedule"},
		{"sm10.hint", []any{"$1,711", "$139", "Babylist"}, "About $1,711 more than your usual $139 at Babylist"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := b.T(English, tc.key, tc.args...); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
