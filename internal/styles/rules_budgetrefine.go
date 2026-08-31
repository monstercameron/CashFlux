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
		gap("0.5rem 1.1rem"),
		flexWrap("wrap"),
		marginLeft("auto"),
	)
	// Two groups, split by CONSEQUENCE. `.budget-hero-acts` holds the controls that
	// do something — filter the list, change your budgets — and keeps button weight.
	// `.budget-hero-meta` holds what you only read or follow: a link off the page and
	// a statistic. They were peers in one flat row, so nothing distinguished a
	// control that writes to your data from a report link beside it.
	rule(".budget-hero-acts, .budget-hero-meta",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
	)
	rule(".budget-hero-acts",
		gap("0.45rem"),
	)
	// The passive group is pushed to the far edge, set quieter, and given a hairline
	// so the boundary is visible without a second border or a heavier divider.
	rule(".budget-hero-meta",
		gap("0.75rem"),
		marginLeft("auto"),
		paddingLeft("1.1rem"),
		borderLeft("1px solid var(--border)"),
		color("var(--text-faint)"),
	)
	// Below the two-column threshold the groups stack, and a left border on a
	// full-width row reads as an indent rather than a divider.
	ruleMedia("(max-width: 720px)", ".budget-hero-meta",
		marginLeft("0"),
		paddingLeft("0"),
		borderLeft("0"),
	)
	// The attention chip shares the .btn-tool FOOTPRINT of the controls beside it —
	// same 38px height, same radius, same padding — so the action group reads as one
	// row of one kind of thing. It was a 999px amber pill next to two rectangular
	// buttons, which is three shapes for three controls that all just do something
	// (Cam, 2026-08-31). It keeps a warm tone, because it is still the one item that
	// reports a problem; the shape carries "control", the colour carries "warning".
	rule(".budget-hero-attn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		minHeight("38px"),
		padding("0.35rem 0.75rem"),
		border("1px solid color-mix(in srgb, var(--warn, #d97706) 45%, var(--border))"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--warn, #d97706) 8%, transparent)"),
		color("var(--warn, #d97706)"),
		font("inherit"),
		fontSize("var(--type-13)"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard)"),
	)
	rule(".budget-hero-attn:hover",
		background("color-mix(in srgb, var(--warn, #d97706) 15%, transparent)"),
	)
	rule(".budget-hero-attn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// Meta-group links: navigation, so they read as text with an underline on hover
	// rather than borrowing the bordered box that means "this edits something".
	rule(".budget-meta-link",
		color("inherit"),
		prop("text-decoration", "none"),
		whiteSpace("nowrap"),
	)
	rule(".budget-meta-link:hover",
		color("var(--text)"),
		prop("text-decoration", "underline"),
		prop("text-underline-offset", "2px"),
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
