// SPDX-License-Identifier: MIT

package styles

// registerTV7B keeps category labels readable beside the proportional summary bar.
func registerTV7B() {
	rule(".txn-category-breakdown", flex("1 1 0"), minWidth("0"))
	rule(".txn-category-list", flexWrap("nowrap"), overflow("visible"), whiteSpace("normal"))
	rule(".txn-category-item", flex("0 1 auto"), width("auto"), maxWidth("none"), minWidth("0"))
	rule(".txn-category-item:nth-child(n+4):not(.txn-category-other)", display("none"))
	rule(".txn-category-breakdown.is-expanded .txn-category-item", display("flex"))
	rule(".txn-category-more", flex("0 0 auto"), padding("0.15rem 0.3rem"), border("0"), background("transparent"), color("var(--accent)"), font("inherit"), cursor("pointer"), whiteSpace("nowrap"))
	rule(".txn-category-segment", pointerEvents("none"))
	rule(".txn-count-future-band", minHeight("44px"), maxHeight("64px"), fontSize("17px"), fontWeight("600"), fontVariantNumeric("tabular-nums"))
	ruleMedia("(max-width: 1180px)", ".txn-count-band", flexWrap("wrap"), maxHeight("none"), gap("0.5rem 0.75rem"))
	ruleMedia("(max-width: 1180px)", ".txn-category-breakdown", flexBasis("100%"), order("3"))
	ruleMedia("(max-width: 1180px)", ".txn-category-bar", order("0"))
	ruleMedia("(max-width: 1180px)", ".txn-category-list", flexWrap("wrap"), overflow("visible"), whiteSpace("normal"))
}
