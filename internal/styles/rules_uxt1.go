// SPDX-License-Identifier: MIT

package styles

// registerUXT1 makes the period totals the transaction page's main visual
// anchor while keeping the row count and other context quiet.
func registerUXT1() {
	rule(".txn-countline-wrap",
		width("100%"),
		minWidth("0"),
	)
	rule(".txn-countline",
		display("flex"),
		flexDirection("column"),
		gap("0.65rem"),
		width("100%"),
		minWidth("0"),
		color("var(--text)"),
		fontSize("var(--type-14)"),
	)
	rule(".txn-countline-future",
		flexDirection("row"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.35rem 1rem"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
	)
	rule(".txn-count-future-period",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".txn-count-future-fact",
		fontWeight("500"),
	)
	rule(".txn-count-figures",
		display("grid"),
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		gap("1rem"),
		width("100%"),
	)
	rule(".txn-count-figure",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.15rem"),
		minWidth("0"),
		padding("0"),
		border("0"),
		background("transparent"),
		color("var(--text)"),
		textAlign("left"),
		font("inherit"),
	)
	rule(".txn-count-figure-button",
		cursor("pointer"),
	)
	rule(".txn-count-figure-button:focus-visible, .txn-category-segment:focus-visible, .txn-category-item:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("3px"),
	)
	rule(".txn-count-value",
		fontSize("17px"),
		fontWeight("600"),
		lineHeight("1.25"),
		fontVariantNumeric("tabular-nums"),
		fontFeatureSettings("tnum"),
		whiteSpace("nowrap"),
	)
	rule(".txn-count-label",
		fontSize("12px"),
		fontWeight("500"),
		lineHeight("1.3"),
		color("var(--text-dim)"),
	)
	rule(".txn-count-negative",
		color("var(--danger, #d95757)"),
	)
	rule(".txn-count-positive",
		color("var(--success, #3fa56e)"),
	)
	rule(".txn-category-breakdown",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		maxWidth("calc((100% - 2rem) / 3)"),
	)
	rule(".txn-category-bar",
		display("flex"),
		width("100%"),
		height("4px"),
		minHeight("4px"),
		overflow("hidden"),
		borderRadius("999px"),
		background("var(--border)"),
	)
	rule(".txn-category-segment",
		display("block"),
		flex("0 0 auto"),
		minWidth("2px"),
		height("4px"),
		padding("0"),
		border("0"),
		borderRadius("0"),
		background("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".txn-category-segment:nth-child(2)", background("color-mix(in srgb, var(--text-dim) 82%, var(--bg) 18%)"))
	rule(".txn-category-segment:nth-child(3)", background("color-mix(in srgb, var(--text-dim) 66%, var(--bg) 34%)"))
	rule(".txn-category-segment:nth-child(4)", background("color-mix(in srgb, var(--text-dim) 50%, var(--bg) 50%)"))
	rule(".txn-category-segment:nth-child(5)", background("color-mix(in srgb, var(--text-dim) 34%, var(--bg) 66%)"))
	rule(".txn-category-segment.is-selected",
		background("var(--accent)"),
	)
	rule(".txn-category-segment:hover",
		background("var(--accent)"),
	)
	rule(".txn-category-list",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".txn-category-item",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.5rem"),
		width("100%"),
		padding("0.08rem 0"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("var(--type-12)"),
		fontWeight("500"),
		textAlign("left"),
		cursor("pointer"),
	)
	rule(".txn-category-item.is-selected, .txn-category-item:hover",
		color("var(--accent)"),
	)
	rule(".txn-category-name",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".txn-category-amount",
		flexShrink("0"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".txn-count-secondary",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.35rem 1rem"),
		minWidth("0"),
		color("var(--text-dim)"),
		fontSize("12px"),
		fontWeight("500"),
		lineHeight("1.4"),
	)
	rule(".txn-count-fact",
		display("inline-flex"),
		alignItems("center"),
		minWidth("0"),
	)
	rule(".txn-countline .txn-count-search-widen",
		padding("0"),
		font("inherit"),
		fontSize("inherit"),
		fontWeight("600"),
	)
	rule(".txn-count-upcoming-fact .txn-count-upcoming",
		display("inline-flex"),
		margin("0"),
		padding("0"),
		border("0"),
		fontSize("inherit"),
	)

	ruleMedia("(max-width: 1024px)", ".txn-count-figures",
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		gap("0.75rem"),
	)
	ruleMedia("(max-width: 1024px)", ".txn-category-breakdown",
		maxWidth("calc((100% - 1.5rem) / 3)"),
	)
	ruleMedia("(max-width: 560px)", ".txn-count-figures",
		gridTemplateColumns("1fr"),
		gap("0.65rem"),
	)
	ruleMedia("(max-width: 560px)", ".txn-count-figure",
		flexDirection("row"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.75rem"),
	)
	ruleMedia("(max-width: 560px)", ".txn-category-breakdown",
		maxWidth("100%"),
	)
}
