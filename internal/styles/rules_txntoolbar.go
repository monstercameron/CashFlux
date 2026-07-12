// SPDX-License-Identifier: MIT

package styles

// registerTxnToolbar emits the sleek transactions-toolbar icon buttons: fixed-size glyph
// buttons (.tbar-btn) that read as an even row, each revealing its text label on hover /
// focus via a small styled tooltip (.tbar-tip) instead of carrying an inline, uneven-
// width text label.
func registerTxnToolbar() {
	rule(".tbar-btn",
		position("relative"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("2.25rem"),
		height("2.25rem"),
		padding("0"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		transition("background .15s ease, border-color .15s ease, color .15s ease"),
	)
	rule(".tbar-btn:hover",
		background("var(--hover)"),
		borderColor("var(--text-dim)"),
	)
	rule(".tbar-btn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// Primary variant — the "Add" action carries the accent so the main action still
	// stands out at a glance.
	rule(".tbar-btn.primary",
		background("var(--accent)"),
		borderColor("var(--accent)"),
		color("#fff"),
	)
	rule(".tbar-btn.primary:hover",
		background("#276f47"),
		borderColor("#276f47"),
	)
	// Danger variant — destructive actions (e.g. bulk delete).
	rule(".tbar-btn.danger",
		color("var(--danger)"),
	)
	rule(".tbar-btn.danger:hover",
		background("rgba(216,113,111,0.12)"),
		borderColor("var(--danger)"),
	)
	// Open state — a glyph button whose flip modal / panel is currently open stays
	// highlighted (accent tint) until it's dismissed, so the trigger reads as active.
	rule(".tbar-btn.open",
		background("var(--accent-dim)"),
		borderColor("var(--accent)"),
		color("var(--accent)"),
	)

	// Hover/focus tooltip revealing the action's label below the glyph.
	rule(".tbar-tip",
		position("absolute"),
		top("calc(100% + 6px)"),
		left("50%"),
		transform("translateX(-50%) translateY(-2px)"),
		padding(".25rem .5rem"),
		borderRadius("6px"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		border("1px solid var(--border)"),
		fontSize(".72rem"),
		whiteSpace("nowrap"),
		pointerEvents("none"),
		opacity("0"),
		transition("opacity .12s ease, transform .12s ease"),
		zIndex("var(--z-popover)"),
		boxShadow("0 4px 12px rgba(0,0,0,.25)"),
	)
	rule(".tbar-btn:hover .tbar-tip, .tbar-btn:focus-visible .tbar-tip",
		opacity("1"),
		transform("translateX(-50%) translateY(0)"),
	)

	// Filters glyph trigger: the active-filter count as a corner badge, and an accent
	// tint on the glyph when any filter is active.
	rule(".tbar-btn.filters-trigger .filter-badge",
		position("absolute"),
		top("-5px"),
		right("-5px"),
		minWidth("1rem"),
		height("1rem"),
		padding("0 .25rem"),
		borderRadius("999px"),
		background("var(--accent)"),
		color("#fff"),
		fontSize(".6rem"),
		fontWeight("700"),
		lineHeight("1"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".tbar-btn.filters-trigger.active",
		borderColor("var(--accent)"),
		color("var(--accent)"),
	)

	// Bulk-action bar: one compact flat row. The category/member selects are capped
	// narrow so the whole row fits without a horizontal scrollbar.
	rule(".bulk-bar",
		flexWrap("nowrap"),
		overflowX("auto"),
		paddingBottom(".2rem"),
	)
	rule(".bulk-bar select",
		maxWidth("150px"),
		minWidth("0"),
	)

	// Tooltip stacking: a ledger tile establishes a transform stacking context on hover
	// (the lift), which would otherwise trap a .tbar-tip below the tile beneath it. Lift
	// the hovered tile's layer above its siblings so the tooltip paints on top.
	rule(".bento-ledger > .w:hover",
		position("relative"),
		zIndex("5"),
	)

	// Redesigned filter panel: each categorical dimension is a labelled group of toggle
	// pills (multi-select), then the date/amount ranges.
	rule(".filter-panel",
		display("flex"),
		flexDirection("column"),
		gap("1.25rem"),
	)
	rule(".filter-groups",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	rule(".filter-group",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".filter-group-label",
		fontSize("0.7rem"),
		fontWeight("600"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".filter-pills",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem"),
		maxHeight("8.5rem"),
		overflowY("auto"),
	)
	rule(".filter-pill",
		padding("0.3rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		fontSize("0.82rem"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("background .12s ease, border-color .12s ease, color .12s ease"),
	)
	rule(".filter-pill:hover",
		borderColor("var(--text-dim)"),
	)
	rule(".filter-pill.on",
		background("var(--accent)"),
		borderColor("var(--accent)"),
		color("#fff"),
	)
	rule(".filter-pill:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".filter-ranges",
		display("flex"),
		flexWrap("wrap"),
		gap("0.75rem"),
		alignItems("flex-end"),
	)
}
