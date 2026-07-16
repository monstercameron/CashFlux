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
}
