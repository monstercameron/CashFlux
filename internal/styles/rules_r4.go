// SPDX-License-Identifier: MIT

package styles

// registerR4Surface holds the round-4 granular spacing / small-element fixes. Registered
// after registerGenerated() so these overrides win the cascade.
func registerR4Surface() {
	// Title-less surface tiles now skip their empty header (see internal/ui/widget.go),
	// removing ~20px of dead space at the top of every surface-page tile. The body picks
	// up a small top padding so its content isn't flush against the tile's top border.
	rule(".wbody.wbody-nohead",
		paddingTop("0.7rem"),
	)

	// The "?" smart-tooltip renders through IconButton, which prepends the full `.btn`
	// base — giving it a ~41px bordered square next to a ~14px caps label on the hero /
	// budgets / goals stat labels (only the `.stat-label` scope shrank it, so Accounts was
	// fine but Dashboard/Budgets/Goals showed a floating tile). Make `.btn-icon-bare`
	// genuinely bare everywhere so the help glyph is a small inline affordance.
	rule(".btn-icon-bare",
		padding("0"),
		border("0"),
		background("transparent"),
		minHeight("0"),
		height("auto"),
		width("auto"),
		lineHeight("1"),
		boxShadow("none"),
	)
	rule(".btn-icon-bare svg",
		width("0.95em"),
		height("0.95em"),
	)

	// Budget cards span the FULL content width (one per row) instead of packing 3-up —
	// each budget reads as a full line. Size to content (drop the fixed min-height that
	// left a tall half-empty box at full width).
	rule(".bento-budgets .budget-grid",
		gridTemplateColumns("1fr"),
	)
	rule(".bento-budgets .budget",
		minHeight("0"),
	)

	// The full-width budget card's quick-metric strip: a scannable row of small
	// labelled figures (Left/day · Days left · Elapsed), set off with a hairline so it
	// reads as at-a-glance stats distinct from the prose sub-lines above it.
	rule(".budget-metrics",
		display("flex"),
		flexWrap("wrap"),
		gap("1.75rem"),
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 60%, transparent)"),
	)
	rule(".budget-metric",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		minWidth("0"),
	)
	rule(".budget-metric-label",
		fontSize("0.62rem"),
		fontWeight("600"),
		letterSpacing("0.07em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".budget-metric-value",
		fontSize("1.02rem"),
		fontWeight("700"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
		lineHeight("1.15"),
	)
	rule(".budget-metric-sub",
		fontSize("0.68rem"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	// Budget list name filter: a compact search field with a leading magnifier, sitting
	// above the card grid. Shown only for longer lists (gated in the tile).
	rule(".budget-search",
		position("relative"),
		display("flex"),
		alignItems("center"),
		marginBottom("0.75rem"),
	)
	rule(".budget-search-icon",
		position("absolute"),
		left("0.6rem"),
		color("var(--text-faint)"),
		pointerEvents("none"),
	)
	rule(".budget-search-input",
		width("100%"),
		paddingLeft("2rem"),
	)
	rule(".budget-search-empty",
		padding("0.75rem 0.25rem"),
		color("var(--text-dim)"),
		fontSize("0.9rem"),
	)
	// Place the annual-grid cell after the budget list (the generated per-tile `order`
	// rules only cover summary/toolbar/list/savings/formula, so a new tile would default
	// to order:0 and jump to the top). Renumber savings/formula to sit after it.
	rule(".bento-budgets > .w[data-widget=\"budget-annualgrid\"]", order("4"))
	rule(".bento-budgets > .w[data-widget=\"budget-savings\"]", order("5"))
	rule(".bento-budgets > .w[data-widget=\"budget-formula\"]", order("6"))
	// A budget's cross-category tracked-tag caption: a label + small #tag chips.
	rule(".budget-tag-line",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.3rem"),
	)
	rule(".budget-tag-line-label",
		color("var(--text-faint)"),
	)
	rule(".budget-tag-chip",
		fontSize("0.72rem"),
		lineHeight("1"),
		padding("0.15rem 0.45rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	// This-month selection metadata in the tracked-categories/tags editor.
	rule(".budgetcat-meta",
		marginLeft("auto"),
		fontSize("0.72rem"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	// A checked/also tag can push the meta; keep the also-note after it.
	rule(".budgetcat-row .budgetcat-meta + .budgetcat-also",
		marginLeft("0.5rem"),
	)
	// The tracked-categories/tags editor: two labelled sections (Categories, Tags).
	rule(".budgettrack-section",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".budgettrack-head",
		fontSize("0.72rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing("0.04em"),
		color("var(--text-faint)"),
	)
	// Cap each section's checklist so BOTH the categories and tags sections (and the
	// footer) stay visible — each list scrolls internally instead of one long column that
	// pushes the tags section off-screen.
	rule(".budgettrack-section .budgetcats-list",
		maxHeight("30vh"),
		overflowY("auto"),
	)
	// "Add a new tag" row when the search matches no existing tag.
	rule(".budgettag-add",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		width("100%"),
		padding("0.45rem 0.5rem"),
		borderRadius("8px"),
		border("1px dashed var(--border-strong)"),
		background("transparent"),
		color("var(--accent)"),
		fontSize("0.85rem"),
		cursor("pointer"),
		textAlign("left"),
	)
	rule(".budgettag-add:hover, .budgettag-add:focus-visible",
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
	)
}
