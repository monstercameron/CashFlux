// SPDX-License-Identifier: MIT

package styles

// registerForm3 gives Add and Edit transactions one responsive, two-column field grid.
func registerForm3() {
	// Add and Edit share this form, including the quick-add sentence prompt. Keep
	// labels on the same body step instead of letting the prompt copy grow larger.
	rule(".txn-form-add, .txn-form-edit",
		fontFamily("Inter, sans-serif"),
	)
	rule(".txn-form-add .qa-sentence > label",
		fontSize("var(--fs-body)"),
		fontWeight("500"),
		fontStyle("normal"),
		textTransform("none"),
	)
	rule(".txn-form-grid",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		gap("var(--space-4)"),
		alignItems("start"),
	)
	rule(".txn-form-grid > *", minWidth("0"))
	rule(".txn-form-grid > .txn-form-sentence, .txn-form-grid > .txn-form-details, .txn-form-grid > .txn-edit-row-actions, .txn-form-grid > .txn-edit-summary, .txn-form-grid > .txt-zone",
		gridColumn("1 / -1"),
	)
	rule(".txn-form-grid .cf-field, .txn-form-grid .field, .txn-form-grid select",
		boxSizing("border-box"),
		width("100%"),
		minWidth("0"),
	)
	rule(".txn-form-grid .txn-cat-row > :first-child",
		flex("1 1 0"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 640px)", ".txn-form-grid",
		gridTemplateColumns("minmax(0, 1fr)"),
	)
}
