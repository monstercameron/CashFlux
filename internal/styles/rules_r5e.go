// SPDX-License-Identifier: MIT

package styles

// registerR5E polishes the shared transaction direction control and the Edit
// category suggestion/action menu.
func registerR5E() {
	rule(".txn-dir-toggle > .txn-dir-option",
		minHeight("2.25rem"),
		flex("1 1 0"),
		transition("background-color 120ms ease, border-color 120ms ease, color 120ms ease"),
	)
	rule(".txn-shared-row",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		gap("0.7rem 0.9rem"),
		alignItems("start"),
		gridColumn("1 / -1"),
	)
	rule(".txn-shared-row .txn-dir-toggle",
		alignSelf("end"),
	)
	rule(".txn-dir-toggle > .txn-dir-option.is-pressed, .qa-grid .btn-group > [aria-pressed=\"true\"]",
		background("color-mix(in srgb, var(--accent) 16%, var(--bg-elev))"),
		borderColor("var(--accent)"),
		color("var(--text)"),
		fontWeight("700"),
		boxShadow("inset 0 0 0 1px var(--accent)"),
	)
	rule(".txn-dir-toggle > .txn-dir-option.is-pressed:hover, .qa-grid .btn-group > [aria-pressed=\"true\"]:hover",
		background("color-mix(in srgb, var(--accent) 23%, var(--bg-elev))"),
		borderColor("var(--accent)"),
	)
	rule(".txn-dir-toggle > .txn-dir-option:focus-visible, .qa-grid .btn-group > [aria-pressed=\"true\"]:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".txn-edit-category-label",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		marginBottom("0.35rem"),
	)
	rule(".txn-edit-category-label .t-caption",
		margin("0"),
	)
	rule(".txn-edit-category-suggestion",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		minHeight("2rem"),
		padding("0.3rem 0.65rem"),
		border("1px solid color-mix(in srgb, var(--accent) 55%, var(--border))"),
		borderRadius("var(--radius-sm, 6px)"),
		background("color-mix(in srgb, var(--accent) 9%, transparent)"),
		color("var(--accent)"),
		fontSize("var(--type-12)"),
		fontWeight("700"),
		whiteSpace("nowrap"),
		cursor("pointer"),
	)
	rule(".txn-edit-category-suggestion:hover",
		background("color-mix(in srgb, var(--accent) 16%, transparent)"),
	)
	rule(".txn-edit-category-suggestion:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".txn-edit-row-actions",
		display("flex"),
		alignItems("center"),
		minHeight("2.25rem"),
	)
	rule(".txn-edit-category-field .txn-cat-new",
		width("2.25rem"),
		minWidth("2.25rem"),
		height("2.25rem"),
		padding("0"),
		justifyContent("center"),
	)
	rule(".txn-edit-danger",
		display("flex"),
		justifyContent("flex-start"),
		gridColumn("1 / -1"),
	)
	rule(".txn-edit-delete",
		minHeight("2.25rem"),
		padding("0.3rem 0.55rem"),
		color("var(--money-negative)"),
		gap("0.4rem"),
		transition("background-color 120ms ease, color 120ms ease, border-color 120ms ease"),
	)
	rule(".txn-edit-delete:hover",
		color("var(--money-negative)"),
		background("color-mix(in srgb, var(--money-negative) 12%, transparent)"),
		borderColor("color-mix(in srgb, var(--money-negative) 48%, var(--border))"),
	)
	rule(".txn-edit-delete:focus-visible",
		outline("2px solid var(--money-negative)"),
		outlineOffset("2px"),
	)
	rule(".set-foot > .txn-edit-delete",
		marginRight("auto"),
	)
}
