// SPDX-License-Identifier: MIT

package styles

// registerTxnEditSurface emits the styles unique to the Edit-transaction flip modal
// that aren't covered by the generated .txn-edit rules — currently the inline
// quick-add category picker: the select + "New category" button on one row, and the
// reveal-on-demand name field + Add/Cancel below it. Theme tokens only.
func registerTxnEditSurface() {
	// The category picker stacks the select row over the (optional) inline add row.
	rule(".txn-cat-picker",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	// Row 1: the category select grows, the "New category" button trails it.
	rule(".txn-cat-row",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".txn-cat-row .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".txn-cat-new",
		flexShrink("0"),
		whiteSpace("nowrap"),
	)
	// Row 2 (revealed): name field grows, Add/Cancel stay their natural size.
	rule(".txn-cat-add",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".txn-cat-add .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".txn-cat-add .btn",
		flexShrink("0"),
	)
}
