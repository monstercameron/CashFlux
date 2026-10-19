// SPDX-License-Identifier: MIT

package styles

// registerBV6H applies the budget campaign's type scale, contrast and target sizing.
func registerBV6H() {
	rule(".bento-budgets",
		prop("--fs-meta", "12.5px"), prop("--fs-body", "14px"), prop("--fs-section", "15px"),
		prop("--fs-title", "18px"), prop("--fs-figure", "28px"),
	)
	rule(".bento-budgets h1, .bento-budgets .budlist-title", fontSize("var(--fs-title)"), fontWeight("600"))
	rule(".bento-budgets h2, .bento-budgets .brc-head", fontSize("var(--fs-section)"), fontWeight("600"))
	rule(".bento-budgets .bud3-headline-figure", fontSize("var(--fs-figure)"), fontWeight("600"), fontVariantNumeric("tabular-nums"))
	rule(".bento-budgets .bud3-headline-secondary", fontSize("var(--fs-body)"), fontWeight("400"))
	rule(".bento-budgets .budget-crow-name, .bento-budgets .budget-name", fontSize("var(--fs-body)"), fontWeight("600"))
	rule(".bento-budgets .budget-crow-amt, .bento-budgets .budget-crow-left, .bento-budgets .budget-card-loader-figs", fontSize("var(--fs-body)"), fontWeight("500"), fontVariantNumeric("tabular-nums"))
	rule(".bento-budgets .budget-sub, .bento-budgets .budget-crow-chip, .bento-budgets .bud-set-lbl, .bento-budgets .bud-set-hint, .bento-budgets .brc-meta", fontSize("var(--fs-meta)"), fontWeight("400"))
	rule(".bento-budgets .bud3-attention-link", color("#28613f"))
	rule(".bento-budgets .budget-agemoney-head .smart-tip-wrap button", minWidth("32px"), minHeight("32px"), gap("0.25rem"), padding("0 0.35rem"))
	rule(".bento-budgets .budget-agemoney-head .smart-tip-wrap button::after", content("\"Why?\""), fontSize("var(--fs-meta)"), fontWeight("500"), color("var(--text)"))
	rule(".bento-budgets .budget-crow-actions", order("2"))
	rule(".bento-budgets .brc-excluded-list", margin("0"), paddingLeft("1.25rem"), color("var(--text)"))
	rule(".bento-budgets .brc-excluded-list li", fontSize("14px"), lineHeight("1.4"))
	rule(".bento-budgets .budget-unbudgeted-chips", listStyle("none"), margin("0"), padding("0"), display("flex"), flexWrap("wrap"), gap("0.5rem"))
	rule(".bento-budgets .brc-rows", listStyle("none"), margin("0"), padding("0"))
	rule(".bento-budgets .budget-unbudgeted-title", margin("0"), fontSize("var(--fs-section)"), fontWeight("600"))
	rule(".bento-budgets .brc-section-title", margin("0"), fontSize("var(--fs-section)"), fontWeight("600"))
	rule(".bento-budgets .budget-rollover-badge", position("relative"))
	rule(".bento-budgets .budget-rollover-popover", position("absolute"), top("calc(100% + 0.35rem)"), left("0"), zIndex("20"), width("min(20rem, 80vw)"), padding("0.75rem"), border("1px solid var(--border)"), borderRadius("var(--radius-md)"), background("var(--bg-card)"), color("var(--text)"), boxShadow("var(--shadow-1)"))
	rule(".bento-budgets .budget-rollover-popover-title", display("block"), marginBottom("0.25rem"), fontWeight("600"))
	rule(".bento-budgets .budget-rollover-popover-text", margin("0"), fontSize("var(--fs-meta)"), lineHeight("1.45"), fontWeight("400"))
}
