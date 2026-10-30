// SPDX-License-Identifier: MIT

package styles

// registerPageHeadline styles the shared page summary that opens Budgets, Transactions
// and Accounts: a large figure with its sentence beside it, an optional gauge, a quiet
// row of supporting figures, and an optional attention line.
func registerPageHeadline() {
	// One type scale for the page summaries. The tokens are defined on the headline and
	// the transactions count line, so Accounts, Budgets and Transactions read the same sizes.
	rule(".page-headline, .txn-countline",
		prop("--fs-meta", "12.5px"), prop("--fs-body", "14px"), prop("--fs-figure", "28px"),
	)
	rule(".page-headline",
		display("flex"), flexDirection("column"), gap("0.7rem"), minWidth("0"),
	)
	rule(".page-headline-copy",
		display("flex"), alignItems("baseline"), flexWrap("wrap"), gap("0.35rem 0.75rem"), minWidth("0"),
	)
	rule(".page-headline-figure",
		fontSize("var(--fs-figure)"), fontWeight("600"), fontVariantNumeric("tabular-nums"), lineHeight("1.15"), color("var(--text)"),
	)
	rule(".page-headline-secondary",
		display("inline-flex"), flexWrap("wrap"), alignItems("baseline"), gap("0.15rem 0.45rem"),
		fontSize("var(--fs-body)"), fontWeight("400"), lineHeight("1.4"), color("var(--text)"),
	)
	rule(".page-headline-secondary .stat-sub",
		display("inline-flex"), flexDirection("row"), alignItems("center"), gap("0.3rem"),
	)
	rule(".page-headline + .t-caption",
		marginTop("0.6rem"),
	)
	rule(".page-headline-aside",
		fontSize("var(--fs-meta)"), fontWeight("400"), lineHeight("1.35"), color("var(--text-dim)"), marginLeft("0.35rem"),
	)
	rule(".page-headline .cf-bar",
		position("relative"),
	)
	rule(".page-headline .cf-bar-pace",
		position("absolute"), top("0"), bottom("0"), width("2px"), zIndex("1"),
		background("var(--text)"), opacity("0.65"), pointerEvents("none"),
	)
	rule(".page-headline-facts",
		display("flex"), alignItems("baseline"), flexWrap("wrap"), gap("0.3rem 1.5rem"),
		fontSize("var(--fs-body)"), color("var(--text)"),
	)
	rule(".page-headline-fact",
		display("inline-flex"), alignItems("baseline"), gap("0.35rem"), color("inherit"),
	)
	rule(".page-headline-fact-link",
		textDecoration("none"),
	)
	rule(".page-headline-fact-label",
		color("var(--text-dim)"),
	)
	rule(".page-headline-fact-value",
		fontWeight("600"), fontVariantNumeric("tabular-nums"),
	)
	rule(".page-headline-attention",
		display("flex"), alignItems("center"), flexWrap("wrap"), gap("0.35rem"),
		fontSize("var(--fs-meta)"), color("var(--text-dim)"),
	)
	rule(".page-headline-attention:empty",
		display("none"),
	)
	rule(".page-headline-top",
		display("flex"), flexWrap("wrap"), alignItems("stretch"), justifyContent("space-between"), gap("0.5rem 1.5rem"), minWidth("0"),
	)
	rule(".page-headline-top > .page-headline-copy",
		flex("1 1 220px"), minWidth("0"),
	)
	rule(".page-headline-visual",
		display("flex"), alignItems("center"), flex("0 1 56%"), minWidth("180px"), maxWidth("560px"), alignSelf("start"),
		prop("mask-image", "linear-gradient(to right, transparent 0, #000 16%, #000 84%, transparent 100%)"),
		prop("-webkit-mask-image", "linear-gradient(to right, transparent 0, #000 16%, #000 84%, transparent 100%)"),
	)
	rule(".page-headline-stack",
		display("flex"), flexDirection("column"), gap("0.6rem"), flex("1 1 220px"), minWidth("0"),
	)
	rule(".page-headline-footer",
		fontSize("var(--fs-meta)"), color("var(--text-dim)"),
	)
	rule(".page-headline-footer:empty",
		display("none"),
	)
	rule(".page-headline-visual:empty",
		display("none"),
	)
	ruleMedia("(max-width: 720px)", ".page-headline-visual",
		flex("1 1 100%"), minWidth("0"), maxWidth("none"),
	)
	rule(".txn-top-actions",
		display("flex"), alignItems("center"), justifyContent("flex-end"), flexWrap("wrap"), gap("0.5rem"),
	)
	rule(".txn-bubble",
		display("flex"), flexDirection("column"), gap("0.6rem"), minWidth("0"),
		background("var(--bg-card)"), border("1px solid var(--border)"), borderRadius("var(--radius-xl)"), padding("0.75rem"),
	)
}
