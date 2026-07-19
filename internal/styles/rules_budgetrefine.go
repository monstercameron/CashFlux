// SPDX-License-Identifier: MIT

package styles

// registerBudgetRefine styles the 2026-07-19 Budgets UX refinement pass: the single
// "Budget settings" popover that now holds the method/sort/compact controls and bulk
// tools, the right-aligned cover-all action on the review-queue head, and the slimmer
// category cards (so the first two budgets clear the fold). Theme tokens only
// (--text / --text-dim / --border / --bg-elev / --accent / --danger), so both light
// and dark track automatically. Registered LAST, so these overrides win at equal
// specificity over the generated card rules.
func registerBudgetRefine() {
	// --- The "Budget settings" popover (one control bar) ---
	// A touch wider than a plain action menu so the two <select> pickers have room.
	rule(".bud-set-menu",
		minWidth("240px"),
	)
	rule(".bud-set-sec",
		display("flex"),
		flexDirection("column"),
		gap("2px"),
	)
	// A quiet uppercase section header ("View" / "Bulk tools").
	rule(".bud-set-head",
		fontSize("0.68rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
		padding("0.3rem 0.5rem 0.1rem"),
	)
	// A labelled picker row: the label stacked over a full-width select.
	rule(".bud-set-field",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		padding("0.3rem 0.5rem"),
	)
	rule(".bud-set-lbl",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	rule(".bud-set-field .fctrl-select",
		width("100%"),
	)
	rule(".bud-set-sep",
		height("1px"),
		background("var(--border)"),
		margin("0.3rem 0.25rem"),
	)

	// --- Review-queue head: the cover-all action sits at the right edge ---
	rule(".bgattn-head-action",
		marginLeft("auto"),
		display("inline-flex"),
		alignItems("center"),
	)

	// --- Slimmer category cards (first two budgets clear the fold) ---
	// Trim the card's vertical padding and drop the fixed min-height, shrink the
	// progress "loader", and tighten the below-bar metadata spacing. Scoped to the
	// budgets bento so nothing else is affected.
	rule(".bento-budgets .budget",
		padding("0.45rem 0"),
		minHeight("0"),
	)
	rule(".bento-budgets .budget-card-loader",
		height("34px"),
		margin("0.3rem 0 0.4rem"),
	)
	rule(".bento-budgets .budget-lower",
		gap("1rem"),
	)
	rule(".bento-budgets .budget-sub",
		marginTop("0.1rem"),
	)
}
