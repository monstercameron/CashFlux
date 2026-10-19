// SPDX-License-Identifier: MIT

package styles

// registerTV8C gives the transaction summary a stable two-line desktop layout
// and keeps the full category list in a readable popover.
func registerTV8C() {
	rule(".txn-count-band",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) auto"),
		alignItems("center"),
		gap("0.5rem 1rem"),
		maxHeight("none"),
	)
	rule(".txn-count-figures", gridColumn("1"), gridRow("1"), minWidth("0"))
	rule(".txn-count-upcoming-fact", gridColumn("2"), gridRow("1"), marginLeft("0"), whiteSpace("nowrap"))
	rule(".txn-category-breakdown", gridColumn("1 / -1"), gridRow("2"), flex("none"), width("100%"), maxWidth("none"), minWidth("0"), position("relative"))
	rule(".txn-category-bar", order("0"), width("100%"))
	rule(".txn-category-list", flexWrap("nowrap"), overflow("visible"), whiteSpace("normal"), gap("1rem"))
	rule(".txn-category-item", flex("0 1 auto"), width("auto"), maxWidth("none"), minWidth("0"), whiteSpace("nowrap"))
	rule(".txn-category-popover",
		position("absolute"),
		zIndex("20"),
		top("100%"),
		left("0"),
		width("min(420px, 100%)"),
		maxHeight("min(360px, 60vh)"),
		overflowY("auto"),
		padding("0.5rem"),
		border("1px solid var(--border)"),
		borderRadius("0.5rem"),
		background("var(--bg-card)"),
		boxShadow("0 8px 24px rgba(0, 0, 0, 0.2)"),
	)
	rule(".txn-category-popover-row", display("grid"), gridTemplateColumns("minmax(0, 1fr) auto"), alignItems("center"), gap("1rem"), minWidth("0"))
	rule(".txn-category-popover .txn-category-item", width("100%"), maxWidth("none"), padding("0.4rem 0.5rem"))
	rule(".txn-category-share", color("var(--text-dim)"), fontSize("var(--fs-meta)"), fontWeight("500"), fontVariantNumeric("tabular-nums"), whiteSpace("nowrap"))
	rule(".txn-count-visible-facts", gridColumn("1 / -1"), width("100%"))
	ruleMedia("(max-width: 760px)", ".txn-count-band", gridTemplateColumns("minmax(0, 1fr)"), maxHeight("none"))
	ruleMedia("(max-width: 760px)", ".txn-count-figures", gridColumn("1"), gridRow("1"))
	ruleMedia("(max-width: 760px)", ".txn-count-upcoming-fact", gridColumn("1"), gridRow("2"), justifySelf("start"))
	ruleMedia("(max-width: 760px)", ".txn-category-breakdown", gridColumn("1"), gridRow("3"))
}
