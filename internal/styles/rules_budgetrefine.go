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
	// Trim the card's VERTICAL padding and drop the fixed min-height, shrink the
	// progress "loader", and tighten the below-bar metadata spacing. Scoped to the
	// budgets bento so nothing else is affected. The horizontal inset must survive
	// the trim: the card still has its own background, radius, and the 5px inset
	// accent edge — `padding: .45rem 0` made the text/loader collide with that edge
	// and clip at the rounded corners (2026-07-19 card-view regression).
	rule(".bento-budgets .budget",
		padding("0.45rem 1rem 0.5rem 1.15rem"),
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

	// --- B1 hero (2026-07-19): the budgets top opens on the SHARED summary band ---
	// (`.budget-loader`, the same component the Goals and To-do headline tiles use)
	// followed by one quiet sub-row: income-received on the left, the action cluster
	// (attention chip · Cover all · Budget income · age chip) on the right.
	rule(".budget-hero",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".budget-hero-side",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
		marginLeft("auto"),
	)
	// The attention chip: the hero row's one warm element; clicking filters the list.
	rule(".budget-hero-attn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		padding("0.25rem 0.7rem"),
		border("1px solid color-mix(in srgb, var(--warn, #d97706) 45%, var(--border))"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--warn, #d97706) 8%, transparent)"),
		color("var(--warn, #d97706)"),
		font("inherit"),
		fontSize("0.78rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard)"),
	)
	rule(".budget-hero-attn:hover",
		background("color-mix(in srgb, var(--warn, #d97706) 15%, transparent)"),
	)
	rule(".budget-hero-toassign",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	rule(".budget-hero-cap",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem 1rem"),
		flexWrap("wrap"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-hero-age",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)

	// --- B1: the list card's head row (search grows, settings/add pin right) ---
	rule(".budlist-head",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginBottom("0.6rem"),
	)
	rule(".budlist-head .budget-search",
		flex("1 1 14rem"),
		minWidth("10rem"),
		marginBottom("0"),
	)
	// The folded toolbar keeps its own classes; neutralize its standalone-band
	// chrome and pin it to the row's right edge.
	rule(".budlist-head .budgets-tb",
		marginLeft("auto"),
		margin("0 0 0 auto"),
		padding("0"),
		border("0"),
		background("transparent"),
	)
	rule(".budlist-head .budgets-tb .filter-toolbar-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		margin("0"),
		padding("0"),
	)
}
