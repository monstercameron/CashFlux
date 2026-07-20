// SPDX-License-Identifier: MIT

package styles

// registerRecapSurface emits the Monthly Recap dashboard banner (CG-S1): a
// distinct accent-barred "month in review" — a period heading, then a row of
// stat cells (spend change, top category, biggest expense, biggest change,
// no-spend days), each with a leading glyph and consistent up/down tone. Theme
// tokens only, so it tracks light/dark.
func registerRecapSurface() {
	// A faint accent tint + left bar sets the banner apart from the neutral tiles
	// around it (and from the "Needs attention" card above), so it reads as its
	// own "here's your month" feature rather than more rows.
	rule(".cf-recap",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
		padding("0.85rem 1rem"),
		borderLeft("3px solid var(--accent)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--accent) 5%, transparent)"),
	)

	rule(".cf-recap-head",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		color("var(--text-dim)"),
	)
	rule(".cf-recap-title",
		fontSize("0.95rem"),
		fontWeight("700"),
		color("var(--text)"),
	)

	rule(".cf-recap-stats",
		display("flex"),
		flexWrap("wrap"),
		alignItems("flex-start"),
		gap("1.25rem"),
	)
	rule(".cf-recap-stat",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
		flex("1 1 8rem"),
	)
	rule(".cf-recap-stat + .cf-recap-stat",
		borderLeft("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
		paddingLeft("1.25rem"),
	)
	// Label row: a small glyph + the uppercase caption.
	rule(".cf-recap-lbl",
		display("flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("var(--type-11)"),
		fontWeight("600"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".cf-recap-val",
		fontSize("1.35rem"),
		fontWeight("700"),
		lineHeight("1.15"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".cf-recap-val.good", color("var(--accent)"))
	rule(".cf-recap-val.bad", color("var(--danger)"))
	rule(".cf-recap-sub",
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
}
