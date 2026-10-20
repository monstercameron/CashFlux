// SPDX-License-Identifier: MIT

package styles

// registerDashboardReadability gives the populated dashboard a clearer reading
// order. It keeps the established color and type system, and scopes the changes
// to the summary hero and dashboard widgets.
func registerDashboardReadability() {
	// Place the headline and its period figures side by side. The previous
	// full-width stacked rows left half the hero unused and pushed action items
	// below the first screen.
	rule(".home-hero--summary",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) minmax(0, 1fr)"),
		prop("column-gap", "2rem"),
		prop("row-gap", "0.4rem"),
		alignItems("start"),
	)
	rule(".home-hero--summary .home-hero-top",
		gridColumn("1"), gridRow("1"), marginBottom("0"),
	)
	rule(".home-hero--summary .home-hero-main",
		gridColumn("1"), gridRow("2"),
		flexDirection("column"), alignItems("stretch"), gap("0.45rem"),
		minWidth("0"), marginBottom("0"),
	)
	rule(".home-hero--summary .home-hero-spark",
		width("100%"), minWidth("0"), maxWidth("none"),
	)
	rule(".home-hero--summary .home-hero-stats-block",
		gridColumn("2"), gridRow("1 / span 2"),
		alignSelf("center"), marginTop("0"), paddingTop("0"),
		minWidth("0"),
	)
	rule(".home-hero--summary .home-hero-stats",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		marginTop("0.35rem"), paddingTop("0"), borderTop("0"),
	)
	rule(".home-hero--summary .home-hero-stat",
		marginRight("0"), padding("0.55rem 0.75rem"),
		borderRight("0"), minWidth("0"),
	)
	rule(".home-hero--summary .home-hero-stat:nth-child(2n)",
		borderLeft("1px solid var(--border)"),
	)
	rule(".home-hero--summary .home-hero-stat:nth-child(n+3)",
		borderTop("1px solid var(--border)"),
	)
	rule(".home-hero--summary .home-hero-stat:last-child",
		padding("0.55rem 0.75rem"),
	)
	rule(".home-hero--summary .home-hero-actions, .home-hero--summary .home-hero-quote",
		gridColumn("1 / -1"),
	)
	rule(".home-hero--summary .home-hero-actions",
		marginTop("0.55rem"),
	)
	// When budgets exist, put the user's daily decision first. The greeting spans
	// both columns; budget pace leads on the left and net worth becomes context.
	rule(".home-hero--budgeted .home-hero-top",
		gridColumn("1 / -1"), gridRow("1"),
	)
	rule(".home-hero--budgeted .home-hero-budget",
		gridColumn("1"), gridRow("2"),
	)
	rule(".home-hero--budgeted .home-hero-main",
		gridColumn("2"), gridRow("2"),
	)

	// Keep muted text readable at normal zoom. Sentence-case labels avoid a row
	// of tiny tracked capitals competing with the actual amounts.
	rule(".home-hero--summary .home-hero-date, .home-hero--summary .home-hero-nw-label, .home-hero--summary .home-hero-stats-window, .home-hero--summary .home-hero-stat-label",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		textTransform("none"), letterSpacing("0"),
	)
	rule(".home-hero--summary .home-hero-stat-value",
		fontSize("1.2rem"),
		fontWeight("650"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".home-hero--summary .dash-preset-desc, .home-hero--summary .dash-preset-sub",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		lineHeight("1.35"),
	)
	rule(".home-hero--summary .dash-preset-sub", display("none"))

	// Budget pace and effective monthly limits use the same figure hierarchy as
	// the rest of the hero, with a thin meter to make pressured limits scannable.
	rule(".home-hero--summary .home-hero-budget",
		display("grid"),
		gridTemplateColumns("minmax(8.5rem, 0.75fr) minmax(0, 1.4fr)"),
		gap("1rem"),
	)
	rule(".home-hero-budget-pace",
		display("flex"), flexDirection("column"), gap("0.3rem"),
		paddingTop("0.2rem"),
	)
	rule(".home-hero-budget-label",
		fontSize("var(--type-13)"), fontWeight("650"),
		color("var(--text-dim)"),
	)
	rule(".home-hero-budget-count",
		fontSize("2rem"), lineHeight("1.05"),
		fontWeight("700"), fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".home-hero-budget-detail",
		fontSize("var(--type-13)"), lineHeight("1.35"),
		color("var(--text-dim)"),
	)
	rule(".home-hero-budget-limits-head, .home-hero-limit-line",
		display("flex"), alignItems("baseline"),
		justifyContent("space-between"), gap("0.5rem"),
	)
	rule(".home-hero-budget-limits-head", marginBottom("0.55rem"))
	rule(".home-hero-budget-link",
		border("0"), padding("0"), background("transparent"),
		fontSize("var(--type-13)"), color("var(--accent)"),
		cursor("pointer"), whiteSpace("nowrap"),
	)
	rule(".home-hero-budget-link:hover",
		textDecoration("underline"), textUnderlineOffset("3px"),
	)
	rule(".home-hero-budget-link:focus-visible",
		outline("2px solid var(--accent)"), outlineOffset("3px"),
	)
	rule(".home-hero-budget-rows",
		display("flex"), flexDirection("column"), gap("0.5rem"),
	)
	rule(".home-hero-limit-row", minWidth("0"))
	rule(".home-hero-limit-name",
		minWidth("0"), overflow("hidden"), textOverflow("ellipsis"),
		whiteSpace("nowrap"), fontSize("var(--type-13)"),
		color("var(--text)"),
	)
	rule(".home-hero-limit-amount",
		fontSize("var(--type-13)"), fontWeight("650"),
		fontVariantNumeric("tabular-nums"), whiteSpace("nowrap"),
		color("var(--text-dim)"),
	)
	rule(".home-hero-limit-track",
		height("4px"), marginTop("0.25rem"),
		borderRadius("var(--radius-pill)"),
		background("color-mix(in srgb, var(--text) 12%, transparent)"),
		overflow("hidden"),
	)
	rule(".home-hero-limit-fill",
		height("100%"), borderRadius("inherit"), background("var(--accent)"),
	)
	rule(".home-hero-limit-fill.is-hot", background("var(--warn)"))

	// A little more vertical room lets the attention and recap widgets show
	// their existing content without turning their labels into 10px captions.
	rule(".bento.no-touch-chrome",
		customProp("--cell", "172px"),
	)
	rule(".bento.no-touch-chrome .attn-group-label",
		fontSize("var(--type-12)"),
		letterSpacing("0.02em"),
		color("var(--text-dim)"),
	)
	rule(".bento.no-touch-chrome .attention-text",
		whiteSpace("normal"),
		prop("display", "-webkit-box"),
		prop("-webkit-box-orient", "vertical"),
		prop("-webkit-line-clamp", "2"),
	)
	rule(".bento.no-touch-chrome .cf-recap-lbl",
		fontSize("var(--type-12)"),
		letterSpacing("0"),
	)
	rule(".bento.no-touch-chrome .cf-recap-sub",
		fontSize("var(--type-13)"),
	)
	rule(".bento.no-touch-chrome .cf-recap",
		gap("0.4rem"), padding("0.4rem 0.85rem"),
	)
	rule(".bento.no-touch-chrome .cf-recap-val",
		fontSize("1.15rem"),
	)
	rule(".bento.no-touch-chrome .w[data-widget=\"monthly-recap\"] .wh h2",
		color("var(--text)"),
	)

	// Below this width the summary reads naturally in one column. The dashboard
	// grid itself already has content-width breakpoints for its widgets.
	ruleMedia("(max-width: 900px)", ".home-hero--summary",
		display("block"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--summary .home-hero-stats-block",
		marginTop("0.9rem"), paddingTop("0.65rem"),
		borderTop("1px solid var(--border)"),
	)
	ruleMedia("(max-width: 1100px)", ".home-hero--summary .home-hero-budget",
		gridTemplateColumns("1fr"), gap("0.8rem"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--budgeted",
		display("flex"), flexDirection("column"), alignItems("stretch"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--budgeted .home-hero-budget",
		order("1"), alignSelf("stretch"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--budgeted .home-hero-main",
		order("2"), marginTop("0.6rem"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--budgeted .home-hero-actions",
		order("3"),
	)
	ruleMedia("(max-width: 900px)", ".home-hero--budgeted .home-hero-quote",
		order("4"),
	)
}
