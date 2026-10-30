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
		gap("0.25rem"),
		width("100%"),
		minWidth("0"),
		color("var(--text)"),
		fontSize("var(--fs-body)"),
	)
	rule(".txn-count-band",
		display("flex"),
		alignItems("center"),
		gap("1rem"),
		width("100%"),
		minWidth("0"),
		minHeight("44px"),
		maxHeight("64px"),
	)
	rule(".txn-countline-future",
		flexDirection("row"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.35rem 1rem"),
		fontSize("var(--fs-meta)"),
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
		display("flex"),
		alignItems("baseline"),
		gap("1rem"),
		flex("0 0 auto"),
	)
	rule(".txn-count-figure",
		display("flex"),
		flexDirection("row"),
		alignItems("baseline"),
		gap("0.35rem"),
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
		fontSize("var(--fs-body)"),
		fontWeight("600"),
		lineHeight("1.25"),
		fontVariantNumeric("tabular-nums"),
		fontFeatureSettings("tnum"),
		whiteSpace("nowrap"),
	)
	rule(".txn-count-label",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		lineHeight("1.3"),
		color("var(--text-dim)"),
	)
	rule(".txn-count-negative",
		color("var(--danger, #d95757)"),
	)
	rule(".txn-count-positive",
		color("var(--positive, var(--up-ink))"),
	)
	rule(".txn-category-breakdown",
		display("flex"),
		flexDirection("column"),
		justifyContent("center"),
		gap("0.3rem"),
		flex("1 1 0"),
		minWidth("0"),
		maxWidth("none"),
	)
	rule(".txn-category-bar",
		display("flex"),
		width("100%"),
		height("6px"),
		minHeight("6px"),
		overflow("hidden"),
		borderRadius("999px"),
		background("var(--border)"),
	)
	rule(".txn-category-segment",
		display("block"),
		flex("0 0 auto"),
		minWidth("0"),
		height("6px"),
		padding("0"),
		border("0"),
		borderRadius("0"),
		background("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".txn-category-segment:nth-child(1)", background("var(--chart-4)"))
	rule(".txn-category-item:nth-child(1)::before", background("var(--chart-4)"))
	rule(".txn-category-segment:nth-child(2)", background("var(--chart-3)"))
	rule(".txn-category-item:nth-child(2)::before", background("var(--chart-3)"))
	rule(".txn-category-segment:nth-child(3)", background("var(--chart-5)"))
	rule(".txn-category-item:nth-child(3)::before", background("var(--chart-5)"))
	rule(".txn-category-segment:nth-child(4)", background("var(--chart-1)"))
	rule(".txn-category-item:nth-child(4)::before", background("var(--chart-1)"))
	rule(".txn-category-segment:nth-child(5)", background("var(--chart-2)"))
	rule(".txn-category-item:nth-child(5)::before", background("var(--chart-2)"))
	rule(".txn-category-segment-other", background("var(--border)"))
	rule(".txn-category-item::before",
		content("\"\""), display("inline-block"), width("8px"), height("8px"), marginRight("0.4rem"), borderRadius("2px"), flex("0 0 auto"),
	)
	rule(".txn-category-segment.is-selected",
		background("var(--accent)"),
	)
	rule(".txn-category-segment.is-dimmed",
		opacity("0.35"),
	)
	rule(".txn-category-bar .txn-category-segment-other, .txn-category-bar .txn-category-segment-other:hover",
		background("var(--border)"),
		cursor("default"),
	)
	rule(".txn-category-segment:hover",
		background("var(--accent)"),
	)
	rule(".txn-category-list",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		minWidth("0"),
		overflow("hidden"),
		whiteSpace("nowrap"),
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
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		textAlign("left"),
		cursor("pointer"),
		maxWidth("22%"),
		whiteSpace("nowrap"),
	)
	rule(".txn-category-item:nth-child(n+4):not(.txn-category-other)", display("none"))
	rule(".txn-category-breakdown.is-filtered .txn-category-item:nth-child(n+4):not(.txn-category-other)", display("flex"))
	rule(".txn-category-breakdown.is-filtered .txn-category-item", maxWidth("16%"))
	rule(".txn-category-item.is-dimmed", opacity("0.45"))
	rule(".txn-category-item.needs-attention .txn-category-name::before",
		content("\"\""),
		display("inline-block"),
		width("6px"),
		height("6px"),
		marginRight("0.3rem"),
		borderRadius("50%"),
		background("var(--warn)"),
		verticalAlign("middle"),
	)
	rule(".txn-category-other",
		cursor("default"),
		color("var(--text-dim)"),
	)
	rule(".txn-category-item.txn-category-other, .txn-category-item.txn-category-other:hover",
		color("var(--text-dim)"),
		cursor("default"),
	)
	rule(".txn-category-more",
		flex("0 0 auto"),
		color("var(--text-dim)"),
		fontSize("var(--fs-meta)"),
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
		fontSize("var(--fs-meta)"),
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
	rule(".txn-count-upcoming-fact",
		flex("0 0 auto"),
		marginLeft("auto"),
		fontSize("var(--fs-meta)"),
	)
	rule(".txn-count-secondary",
		paddingLeft("0"),
	)

	ruleMedia("(max-width: 560px)", ".txn-count-figures",
		gap("0.5rem"),
	)
	ruleMedia("(max-width: 760px)", ".txn-count-band",
		flexWrap("wrap"),
		maxHeight("none"),
		gap("0.5rem 0.75rem"),
	)
	ruleMedia("(max-width: 760px)", ".txn-category-breakdown",
		flexBasis("100%"),
		order("3"),
	)
	ruleMedia("(max-width: 760px)", ".txn-count-upcoming-fact", order("4"))
}
