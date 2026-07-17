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

	// Over-income warning chip on the Budgets summary — the one budget-basis state worth
	// flagging (budgeted more than you earn). A tinted red pill with an icon, so it reads
	// as an alert instead of dim inline text lost in the "income · budgeted · …" line.
	rule(".budget-overincome-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		padding("0.15rem 0.55rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, #ef4444 45%, var(--border))"),
		background("color-mix(in srgb, #ef4444 14%, transparent)"),
		color("var(--text-down)"),
		fontWeight("600"),
	)

	// "What's driving this?" disclosure on a near/over budget card — the analytical link
	// to the charges behind an overspend. Kept deliberately QUIET and monochrome: the card
	// already owns one red signal (the status line), so the driver amounts stay neutral and
	// the toggle reads as a calm caption, not a competing alert.
	rule(".budget-drivers",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		marginTop("0.1rem"),
	)
	rule(".budget-drivers-toggle",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.2rem"),
		alignSelf("flex-start"),
		padding("0"),
		border("none"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		transition("color .12s ease"),
	)
	rule(".budget-drivers-toggle:hover",
		color("var(--text)"),
	)
	rule(".budget-drivers-chev",
		transition("transform .15s ease"),
	)
	rule(".budget-drivers-toggle[aria-expanded=\"true\"] .budget-drivers-chev",
		transform("rotate(180deg)"),
	)
	rule(".budget-drivers-list",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		marginTop("0.1rem"),
	)
	rule(".budget-driver-row",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		textAlign("left"),
		display("flex"),
		alignItems("baseline"),
		gap("0.5rem"),
		width("100%"),
		padding("0.22rem 0.4rem"),
		border("none"),
		borderRadius("6px"),
		background("transparent"),
		color("var(--text)"),
		transition("background .12s ease"),
	)
	rule(".budget-driver-row:hover",
		background("color-mix(in srgb, var(--text) 6%, transparent)"),
	)
	rule(".budget-driver-name",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontSize("0.86rem"),
	)
	rule(".budget-driver-recurring",
		flex("none"),
		fontSize("0.64rem"),
		textTransform("uppercase"),
		letterSpacing("0.05em"),
		color("var(--text-dim)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		padding("0.02rem 0.4rem"),
	)
	rule(".budget-driver-amt",
		flex("none"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
		fontSize("0.86rem"),
	)
	rule(".budget-drivers-empty",
		color("var(--text-faint)"),
		fontSize("0.8rem"),
		margin("0.15rem 0 0"),
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
	// Composite budgets show a spend-composition donut UNDER the full-width status bar: a
	// quiet horizontal strip (donut + amount legend). The bar keeps 100% width.
	rule(".budget-pie",
		display("flex"),
		alignItems("center"),
		gap("1.1rem"),
		margin("0.5rem 0 0.15rem"),
	)
	// Below the bar, a flex row: the status/metrics/actions stack on the left, and — when the
	// budget has a note — the note in a right-hand column (filling the otherwise-empty space).
	rule(".budget-lower",
		display("flex"),
		alignItems("flex-start"),
		gap("1.5rem"),
	)
	rule(".budget-lower-main",
		flex("1 1 auto"),
		minWidth("0"),
	)
	// The right column holds the note (top) and the budget's linked follow-up to-dos (below),
	// each a quiet panel top-aligned with the donut.
	rule(".budget-side-col",
		flex("0 0 clamp(220px, 32%, 360px)"),
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".budget-side-col .budget-notes",
		width("100%"),
		maxWidth("none"),
		marginTop("0"),
		alignItems("flex-start"),
		background("color-mix(in srgb, var(--text) 3%, transparent)"),
	)
	// A note in the panel shows more of itself before its read-more clamp.
	rule(".budget-side-col .budget-notes .acct-notes-text",
		prop("-webkit-line-clamp", "7"),
	)
	// Linked follow-up to-dos panel: a small header over the check-off rows (reusing the
	// transaction follow-up item styling).
	rule(".budget-todos",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		padding("0.5rem 0.55rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("color-mix(in srgb, var(--text) 3%, transparent)"),
	)
	rule(".budget-todos-head",
		fontSize("0.68rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing("0.04em"),
		color("var(--text-faint)"),
		marginBottom("0.15rem"),
	)
	// Left-align the ⋯ overflow: keep it inline with the other action buttons rather than
	// shoved to the far right (overrides the generated margin-left:auto).
	rule(".bento-budgets .budget-actions .add-wrap",
		marginLeft("0"),
	)
	ruleMedia("(max-width: 720px)", ".budget-lower",
		flexDirection("column"),
	)
	rule(".budget-pie-donut",
		position("relative"),
		width("96px"),
		height("96px"),
		borderRadius("50%"),
		flexShrink("0"),
	)
	rule(".budget-pie-hole",
		position("absolute"),
		top("27%"),
		left("27%"),
		right("27%"),
		bottom("27%"),
		borderRadius("50%"),
		background("var(--bg-card)"),
	)
	rule(".budget-pie-legend",
		display("flex"),
		flexDirection("column"),
		gap("0.28rem"),
		minWidth("0"),
	)
	rule(".budget-pie-legrow",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		fontSize("0.75rem"),
	)
	rule(".budget-pie-dot",
		width("9px"),
		height("9px"),
		borderRadius("2px"),
		flexShrink("0"),
	)
	rule(".budget-pie-leglabel",
		color("var(--text-dim)"),
		maxWidth("10rem"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".budget-pie-legval",
		marginLeft("auto"),
		paddingLeft("0.6rem"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
}
