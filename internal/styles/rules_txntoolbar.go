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
}
