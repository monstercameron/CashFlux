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

	// --- B1 hero (2026-07-19): one answer at the top of /budgets ---
	// LEFT-TO-SPEND as the single Fraunces figure, a slim month-ledger bar, one
	// caption line with the age-of-money chip at its right edge. Everything quiet
	// except the number; the attention chip is the only warm element.
	rule(".budget-hero",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".budget-hero-top",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		gap("1rem"),
		flexWrap("wrap"),
	)
	rule(".budget-hero-label",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.68rem"),
		fontWeight("700"),
		letterSpacing("0.07em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".budget-hero-num",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontSize("2.1rem"),
		fontWeight("600"),
		lineHeight("1.05"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".budget-hero-num.pos", color("var(--money-positive)"))
	rule(".budget-hero-num.neg", color("var(--money-negative)"))
	rule(".budget-hero-num.is-warn", color("var(--warn, #d97706)"))
	rule(".budget-hero-side",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
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
	// The month-ledger bar: a 10px track reusing the loader's tone classes.
	rule(".budget-hero-bar",
		position("relative"),
		height("10px"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		overflow("hidden"),
	)
	rule(".budget-hero-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		background("var(--money-positive)"),
	)
	rule(".budget-hero-fill.is-near", background("var(--warn, #d97706)"))
	rule(".budget-hero-fill.is-over", background("var(--money-negative)"))
	rule(".budget-hero-fill.is-hist", background("color-mix(in srgb, var(--text) 35%, transparent)"))
	rule(".budget-hero-cap",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("1rem"),
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
