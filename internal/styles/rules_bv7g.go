// SPDX-License-Identifier: MIT

package styles

// registerBV7G styles the budget dialogs with the shared page type scale and
// gives the month review's leftover summary a readable, aligned layout.
func registerBV7G() {
	rule(".acct-edit-form.cover-all, .mc-body",
		fontSize("var(--fs-body, 14px)"),
		fontWeight("400"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".cover-all .cover-all-row-name, .mc-section-title",
		fontSize("var(--fs-title, 18px)"),
		fontWeight("600"),
	)
	rule(".cover-all .cover-all-row-over, .mc-body .budget-sub, .mc-leftover-head, .mc-leftover-outcome",
		fontSize("var(--fs-meta, 12.5px)"),
		fontWeight("400"),
	)
	rule(".mc-leftovers",
		display("grid"),
		gap("0.35rem"),
	)
	rule(".mc-leftover-head, .mc-leftover-row",
		display("grid"),
		gridTemplateColumns("minmax(8rem, 1.4fr) minmax(5rem, 0.7fr) minmax(8rem, 1fr) minmax(9rem, 1.2fr)"),
		prop("column-gap", "0.8rem"),
		alignItems("center"),
	)
	rule(".mc-leftover-head",
		color("var(--muted)"),
		fontWeight("500"),
	)
	rule(".mc-leftover-row",
		padding("0.45rem 0"),
		borderBottom("1px solid color-mix(in srgb, var(--border) 55%, transparent)"),
	)
	rule(".mc-leftover-amount, .mc-leftover-monthly",
		fontVariantNumeric("tabular-nums"),
		fontWeight("500"),
	)
	rule(".budget-edit .budget-edit-row",
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		prop("column-gap", "1rem"),
		rowGap("0.8rem"),
		alignItems("start"),
	)
	rule(".budget-edit [data-testid='budget-edit-method-hint']",
		display("block"),
		lineHeight("1.4"),
		whiteSpace("normal"),
	)
}
