// SPDX-License-Identifier: MIT

package styles

// registerBR3Head gives Budgets a compact, page-specific reading scale and a
// headline that keeps spending, plan size and the attention filter together.
func registerBR3Head() {
	rule(".bento-budgets h1, .bento-budgets .budlist-title",
		fontSize("20px"), fontWeight("600"), lineHeight("1.25"),
	)
	rule(".bento-budgets h2, .bento-budgets .brc-head",
		fontSize("15px"), fontWeight("600"), lineHeight("1.3"),
	)
	rule(".bento-budgets .bud3-headline",
		display("flex"), flexDirection("column"), gap("0.45rem"), minWidth("0"),
	)
	rule(".bento-budgets .bud3-headline-copy",
		display("flex"), alignItems("baseline"), flexWrap("wrap"), gap("0.25rem 0.55rem"), minWidth("0"),
	)
	rule(".bento-budgets .bud3-headline-figure",
		fontSize("28px"), fontWeight("600"), fontVariantNumeric("tabular-nums"), lineHeight("1.15"), color("var(--text)"),
	)
	rule(".bento-budgets .bud3-headline-secondary",
		fontSize("15px"), fontWeight("400"), lineHeight("1.4"), color("var(--text)"),
	)
	rule(".bento-budgets .bud3-headline-meter",
		position("relative"), height("6px"), minHeight("6px"), width("100%"), overflow("hidden"),
		borderRadius("var(--radius-pill)"), background("var(--bg-elev)"),
	)
	rule(".bento-budgets .bud3-headline-meter-fill",
		height("100%"), borderRadius("inherit"), background("var(--accent)"),
	)
	rule(".bento-budgets .bud3-headline-pace",
		position("absolute"), top("0"), bottom("0"), width("2px"), zIndex("1"),
		background("var(--text)"), opacity("0.65"), pointerEvents("none"),
	)
	rule(".bento-budgets .bud3-headline-attention",
		display("flex"), alignItems("center"), flexWrap("wrap"), gap("0.35rem"),
		fontSize("var(--fs-meta)"), color("var(--text-dim)"),
	)
	rule(".bento-budgets .bud3-attention-separator", color("var(--text-faint)"))
	rule(".bento-budgets .bud3-attention-link",
		minWidth("24px"), minHeight("24px"), padding("0"), border("0"), background("transparent"),
		color("var(--text-dim) !important"), font("inherit"), textDecoration("underline"), textUnderlineOffset("2px"), cursor("pointer"),
	)
	rule(".bento-budgets .bud3-age-explain",
		fontSize("12px"), fontWeight("400"), color("var(--text-dim)"), whiteSpace("nowrap"),
	)
	rule(".bento-budgets .bud-set-head, .bento-budgets .budget-add-or, .bento-budgets .budget-strip-label, .bento-budgets .budget-metric-label, .bento-budgets .brc-total-label, .bento-budgets .bfut-stat-label, .bento-budgets .zbb-sources-title, .bento-budgets .zbb-sources-total-cap, .bento-budgets .zbb-source-kind, .bento-budgets .zbb-alloc-cap-label, .bento-budgets .budget-agemoney-label, .bento-budgets .ba-group-head, .bento-budgets .budget-annualgrid-winlabel, .bento-budgets .autobudget-impact-lbl, .bento-budgets .adjustall-total-lbl, .bento-budgets .budget-limit-basetag, .bento-budgets .bflex-kicker-label, .bento-budgets .bflex-editor-label, .bento-budgets .bflex-ledger-title, .bento-budgets .bflex-badge",
		fontSize("var(--fs-meta)"), fontWeight("400"), letterSpacing("normal"), textTransform("none"), color("var(--text-dim)"),
	)
	rule(".bento-budgets .budget-crow-name, .bento-budgets .budget-name",
		fontSize("14px"), fontWeight("600"),
	)
	rule(".bento-budgets .budget-crow-amt, .bento-budgets .budget-crow-left, .bento-budgets .budget-card-loader-figs",
		fontSize("14px"), fontWeight("500"), fontVariantNumeric("tabular-nums"),
	)
	rule(".bento-budgets .budget-sub, .bento-budgets .budget-crow-chip, .bento-budgets .bud-set-lbl, .bento-budgets .bud-set-hint",
		fontSize("var(--fs-meta)"), fontWeight("400"), lineHeight("1.4"), color("var(--text-dim)"),
	)
	rule(".bento-budgets .budget-add-or", minHeight("24px"))
	rule(".bento-budgets .budget-cat-row .cf-picker-trigger",
		width("100%"), minHeight("40px"), padding("0.5rem 0.75rem"), border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"), background("var(--bg-card)"), color("var(--text)"),
	)
	rule(".bento-budgets .budget-cat-row .cf-picker-trigger::after",
		content("\"⌄\""), marginLeft("auto"), color("var(--text-dim)"), fontSize("16px"),
	)
	ruleMedia("(max-width: 720px)", ".bento-budgets .budlist-head",
		maxHeight("96px"), minHeight("0"), borderBottom("0"),
	)
	for _, selector := range []string{
		".bento-budgets .smart-rowhint-x",
		".bento-budgets .budget-hero-age",
		".topbar .sample-banner-x",
		".topbar .sample-banner-btn",
	} {
		rule(selector, minWidth("24px"), minHeight("24px"), display("inline-flex"), alignItems("center"), justifyContent("center"))
	}
}
