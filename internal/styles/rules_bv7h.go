// SPDX-License-Identifier: MIT

package styles

// registerBV7H applies the final desktop budget type, contrast, and row refinements.
func registerBV7H() {
	rule(".bento-budgets",
		prop("--fs-meta", "12.5px"),
		prop("--fs-body", "14px"),
		prop("--fs-section", "15px"),
		prop("--fs-title", "18px"),
		prop("--fs-figure", "28px"),
	)
	rule(".bento-budgets h1, .bento-budgets .budlist-title",
		fontSize("var(--fs-title)"),
		fontWeight("600"),
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
	)
	rule(".bento-budgets h2, .bento-budgets .brc-head, .bento-budgets .budget-unbudgeted-title, .bento-budgets .brc-section-title",
		fontSize("var(--fs-section)"),
		fontWeight("600"),
	)
	rule(".bento-budgets .budget-crow-name, .bento-budgets .budget-name, .bento-budgets .budget-limit-editor-title",
		fontSize("var(--fs-body)"),
		fontWeight("600"),
	)
	rule(".bento-budgets .bud3-attention-link, .bento-budgets .budget-meta-link, .bento-budgets .budget-unbudgeted-all, .bento-budgets .budget-unbudgeted-cta",
		color("var(--text-dim) !important"),
	)
	rule(".bento-budgets .budget-sub, .bento-budgets .budget-crow-sub, .bento-budgets .brc-meta, .bento-budgets .budget-issue-body, .bento-budgets .bud3-age-explain, .bento-budgets .budget-agemoney-explain",
		color("var(--text-dim) !important"),
		fontSize("var(--fs-meta)"),
		fontWeight("400"),
	)
	rule(".bento-budgets .budget-crow-figures",
		prop("column-gap", "12px"),
		minWidth("260px"),
		flexWrap("nowrap"),
		whiteSpace("nowrap"),
		fontSize("var(--fs-body)"),
	)
	rule(".bento-budgets .budget-crow-fig-separator", display("inline"), marginInline("0"))
	rule(".bento-budgets .budget-cadence", marginLeft("4px"), fontSize("var(--fs-meta)"))
	rule(".bento-budgets .budget-crow",
		gridTemplateColumns("minmax(220px, 1fr) minmax(260px, max-content) minmax(180px, 1.4fr) 110px 72px"),
	)
	rule(".bento-budgets .budget-crow-head", minWidth("220px"), overflow("visible"))
	rule(".bento-budgets .budget-crow-name", minWidth("220px"), overflow("visible"), textOverflow("clip"), whiteSpace("normal"))
	rule(".bento-budgets .budget-crow-chip", maxWidth("100%"), minWidth("0"), flexShrink("1"), overflow("hidden"), textOverflow("ellipsis"), whiteSpace("nowrap"))
	rule(".bento-budgets .budget-rollover-detail th, .bento-budgets .budget-rollover-detail td",
		padding("0.45rem 0.65rem"),
	)
	rule(".bento-budgets .budget-limit-editform",
		display("grid"),
		gridTemplateColumns("minmax(180px, 1fr) auto minmax(180px, 240px) auto auto"),
		alignItems("center"),
		gap("0.5rem 0.75rem"),
		padding("0"),
		border("0"),
		borderRadius("0"),
		background("transparent"),
	)
	rule(".bento-budgets .budget-limit-editor-title", minWidth("0"), fontSize("var(--fs-body)"), fontWeight("600"))
	rule(".bento-budgets .budget-limit-editor-cap, .bento-budgets .budget-limit-editor-base label", fontSize("var(--fs-meta)"), fontWeight("400"))
	rule(".bento-budgets .budget-agemoney-head .smart-tip-wrap button, .bento-budgets .budget-agemoney-why",
		minWidth("24px"), minHeight("24px"),
	)
	rule(".bento-budgets .budget-agemoney-head .smart-tip-wrap button::after", fontSize("var(--fs-meta)"), fontWeight("500"))
	rule(".bento-budgets .budget-issues-toggle", minHeight("24px"), padding("0.45rem 0.7rem"))
	rule(".bento-budgets .budget-strip-label, .bento-budgets .budget-agemoney-label, .bento-budgets .zbb-sources-title, .bento-budgets .zbb-sources-total-cap, .bento-budgets .zbb-source-aside, .bento-budgets .zbb-alloc-cap-label, .bento-budgets .ba-group-head, .bento-budgets .budget-add-or, .bento-budgets .budget-annualgrid-winlabel, .bento-budgets .autobudget-impact-lbl, .bento-budgets .adjustall-total-lbl, .bento-budgets .budget-limit-basetag",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		letterSpacing("normal"),
		textTransform("none"),
		color("var(--text-dim)"),
	)
	ruleMedia("(min-width: 901px) and (max-width: 1100px)", ".bento-budgets .budget-crow-chip",
		maxWidth("100%"),
		flexShrink("1"),
	)
	ruleMedia("(min-width: 901px) and (max-width: 1100px)", ".bento-budgets .budget-crow-figures",
		prop("column-gap", "8px"),
		flexWrap("nowrap"),
		minWidth("0"),
	)
	ruleMedia("(min-width: 901px) and (max-width: 1100px)", ".bento-budgets .budget-crow-amt",
		whiteSpace("nowrap"),
	)
}
