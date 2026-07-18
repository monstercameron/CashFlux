// SPDX-License-Identifier: MIT

package styles

// registerReportsAnnual emits the Annual Review document (/reports redesign): a
// long, dense, single-column report that reads from strengths to problem spots
// to a plan. The signature element is the VERDICT SPINE — every section carries
// a left-edge tone (healthy green → neutral → watch amber → problem red →
// accent-toned plan) so the healthy→unhealthy narrative is literally the page's
// structure. Serif display for verdicts/figures, hairline-ruled tables with
// tabular numerals for the dense data. Theme tokens throughout.
func registerReportsAnnual() {
	// Zone tones, resolved once as CSS vars per section.
	rawBlock(`.rpta-z-up{--rpta-zone:var(--up,#4ea777)}
.rpta-z-neutral{--rpta-zone:var(--border)}
.rpta-z-warn{--rpta-zone:var(--warn,#d8a24a)}
.rpta-z-down{--rpta-zone:var(--down,#d8716f)}
.rpta-z-plan{--rpta-zone:var(--accent)}
.rpta-z-dim{--rpta-zone:var(--border-subtle,var(--border))}
.rpta-tone-up{color:var(--up,#4ea777)}
.rpta-tone-warn{color:var(--warn,#d8a24a)}
.rpta-tone-down{color:var(--down,#d8716f)}
.rpta-fill-up{background:var(--up,#4ea777)}
.rpta-fill-warn{background:var(--warn,#d8a24a)}
.rpta-fill-down{background:var(--down,#d8716f)}
.rpta-dot-up{background:var(--up,#4ea777)}
.rpta-dot-neutral{background:var(--text-faint)}
.rpta-dot-warn{background:var(--warn,#d8a24a)}
.rpta-dot-down{background:var(--down,#d8716f)}
.rpta-dot-plan{background:var(--accent)}`)

	// The document column: fills the content area, generous vertical rhythm.
	rule(".rpta",
		display("flex"),
		flexDirection("column"),
		gap("2.5rem"),
		width("100%"),
		paddingBottom("4rem"),
	)
	rule(".rpta-muted",
		color("var(--text-dim)"),
	)

	// ── Masthead ─────────────────────────────────────────────────────────────
	rule(".rpta-masthead",
		display("flex"),
		flexDirection("column"),
		gap("0.9rem"),
		padding("1.5rem 0 1.75rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rpta-eyebrow",
		margin("0"),
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.14em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".rpta-title",
		margin("0"),
		fontSize("2.3rem"),
		lineHeight("1.1"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".rpta-verdict",
		display("flex"),
		alignItems("baseline"),
		gap("0.9rem"),
		flexWrap("wrap"),
	)
	rule(".rpta-verdict-score",
		fontSize("1.35rem"),
		fontWeight("700"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-verdict-line",
		fontSize("1.02rem"),
		color("var(--text-dim)"),
		maxWidth("48rem"),
	)
	rule(".rpta-figs",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(170px, 1fr))"),
		gap("1.25rem"),
		marginTop("0.5rem"),
	)
	rule(".rpta-fig",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		paddingLeft("0.85rem"),
		borderLeft("2px solid var(--border)"),
	)
	rule(".rpta-fig-k",
		fontSize("0.68rem"),
		fontWeight("700"),
		letterSpacing("0.08em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".rpta-fig-v",
		fontSize("1.55rem"),
		lineHeight("1.1"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".rpta-fig-sub",
		fontSize("0.78rem"),
	)

	// ── Toolbar (tabless) ────────────────────────────────────────────────────
	rule(".rpta-toolbar",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".rpta-toolbar-row",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)

	// ── Sticky jump index ────────────────────────────────────────────────────
	rule(".rpta-index",
		position("sticky"),
		top("var(--topbar-h, 3.5rem)"),
		zIndex("5"),
		display("flex"),
		alignItems("center"),
		gap("0.25rem"),
		flexWrap("wrap"),
		padding("0.5rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-card) 92%, transparent)"),
		prop("backdrop-filter", "blur(6px)"),
	)
	rule(".rpta-idx-item",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		padding("0.25rem 0.55rem"),
		border("none"),
		background("transparent"),
		fontFamily("inherit"),
		cursor("pointer"),
		borderRadius("999px"),
		color("var(--text-dim)"),
		textDecoration("none"),
		fontSize("0.78rem"),
		whiteSpace("nowrap"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".rpta-idx-item:hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".rpta-idx-dot",
		width("7px"),
		height("7px"),
		borderRadius("999px"),
		flexShrink("0"),
	)
	rule(".rpta-idx-num",
		fontSize("0.68rem"),
		fontWeight("700"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	// 2026-07-17 audit: the sticky index must stay ONE compact row. Under 1280px
	// the 11 labeled chips wrapped to two or three sticky rows that followed the
	// reader down the whole report — below that width the labels yield to dot +
	// number (each item keeps its section name in the title tooltip).
	ruleMedia("(max-width: 1280px)", ".rpta-idx-label",
		display("none"),
	)
	ruleMedia("(max-width: 1280px)", ".rpta-index",
		flexWrap("nowrap"),
		overflowX("auto"),
	)

	// ── Sections: the verdict spine ──────────────────────────────────────────
	rule(".rpta-sec",
		borderLeft("3px solid var(--rpta-zone)"),
		paddingLeft("1.4rem"),
		prop("scroll-margin-top", "8.5rem"),
	)
	rule(".rpta-sec-head",
		display("flex"),
		alignItems("flex-start"),
		justifyContent("space-between"),
		gap("1rem"),
		marginBottom("1.1rem"),
	)
	rule(".rpta-sec-title-wrap",
		display("flex"),
		alignItems("baseline"),
		gap("0.85rem"),
	)
	rule(".rpta-sec-num",
		fontSize("2rem"),
		lineHeight("1"),
		fontWeight("600"),
		color("var(--rpta-zone)"),
		fontVariantNumeric("tabular-nums"),
		opacity("0.85"),
	)
	rule(".rpta-z-neutral .rpta-sec-num, .rpta-z-dim .rpta-sec-num",
		color("var(--text-faint)"),
	)
	rule(".rpta-sec-title",
		margin("0"),
		fontSize("1.45rem"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".rpta-sec-sub",
		margin("0.2rem 0 0"),
		fontSize("0.88rem"),
		color("var(--text-dim)"),
		maxWidth("46rem"),
	)
	rule(".rpta-sec-body",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	rule(".rpta-subrow",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		marginTop("0.4rem"),
	)
	rule(".rpta-subhead",
		fontSize("0.72rem"),
		fontWeight("700"),
		letterSpacing("0.1em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".rpta-drill",
		fontSize("0.8rem"),
		color("var(--accent)"),
		textDecoration("none"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-drill:hover",
		textDecoration("underline"),
	)

	// ── Health factor rows ───────────────────────────────────────────────────
	rule(".rpta-facts",
		display("flex"),
		flexDirection("column"),
	)
	rule(".rpta-fact",
		display("grid"),
		gridTemplateColumns("minmax(140px, 1.2fr) minmax(70px, auto) 2fr 2.5rem"),
		alignItems("center"),
		gap("0.9rem"),
		padding("0.5rem 0"),
		borderBottom("1px solid var(--border-subtle, var(--border))"),
	)
	rule(".rpta-fact:last-child", borderBottom("0"))
	rule(".rpta-fact-name",
		fontWeight("600"),
		color("var(--text)"),
	)
	// "Household-wide" tag on sections that deliberately ignore an active
	// report scope — same quiet dashed-pill language as the partial chip.
	rule(".rpta-hh-chip",
		display("inline-flex"),
		alignItems("center"),
		padding("0.1rem 0.55rem"),
		borderRadius("999px"),
		border("1px dashed var(--border)"),
		color("var(--text-dim)"),
		fontSize("0.72rem"),
		fontWeight("600"),
		whiteSpace("nowrap"),
		cursor("help"),
	)
	rule(".rpta-fact-win",
		fontWeight("400"),
		fontSize("0.85em"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-fact-val",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-fact-bar",
		position("relative"),
		height("8px"),
		borderRadius("999px"),
		overflow("hidden"),
		background("color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".rpta-fact-fill",
		position("absolute"),
		top("0"), left("0"), bottom("0"),
		borderRadius("999px"),
	)
	rule(".rpta-fact-score",
		fontVariantNumeric("tabular-nums"),
		fontWeight("700"),
		textAlign("right"),
	)
	// Wins strip: quiet celebratory chips.
	rule(".rpta-wins",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	rule(".rpta-win",
		display("inline-flex"),
		alignItems("center"),
		padding("0.3rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--up, #4ea777) 40%, var(--border))"),
		background("color-mix(in srgb, var(--up, #4ea777) 8%, transparent)"),
		fontSize("0.82rem"),
		color("var(--text)"),
	)

	// ── Money flow: in-house smooth-ribbon sankey + per-$100 companion ───────
	rule(".rpta-sankey",
		width("100%"),
		overflowX("auto"),
	)
	rule(".rpta-flow-host",
		display("block"),
		width("100%"),
	)
	rule(".rpta-flow-svg",
		display("block"),
		width("100%"),
		height("auto"),
	)
	// Labels sit directly on the ribbons; a background-colored stroke halo
	// (paint-order) keeps them readable over any ribbon color in both themes.
	rule(".rpta-flow-svg text",
		fontSize("13px"),
		prop("paint-order", "stroke"),
		prop("stroke", "var(--bg)"),
		prop("stroke-width", "4px"),
		prop("stroke-linejoin", "round"),
	)
	rule(".rpta-flow-name",
		fontWeight("600"),
		fill("var(--text)"),
	)
	rule(".rpta-flow-amt",
		fill("var(--text-dim)"),
	)
	rule(".rpta-flow-link",
		prop("fill-opacity", "0.42"),
		transition("fill-opacity 0.15s ease"),
	)
	rule(".rpta-flow-link:hover",
		prop("fill-opacity", "0.74"),
	)
	rule(".rpta-flow-node",
		prop("fill-opacity", "0.92"),
	)
	rule(".rpta-flow-key",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("1rem"),
		marginTop("0.5rem"),
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	rule(".rpta-flow-key-item",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
	)
	rule(".rpta-flow-dot",
		display("inline-block"),
		width("10px"),
		height("10px"),
		borderRadius("999px"),
		flexShrink("0"),
	)
	rule(".rpta-flow-key-note",
		color("var(--text-faint)"),
		fontStyle("italic"),
	)
	rule(".rpta-chart-legend",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		marginTop("0.35rem"),
		fontSize("0.75rem"),
		color("var(--text-dim)"),
	)
	// Chart plot row: y-axis anchor values beside the plot, so no chart floats
	// without its value scale. The column is exactly the SVG's 120px tall (the
	// x-label row sits below the plot, outside the axis).
	rule(".rpta-chart-body",
		display("flex"),
		gap("0.5rem"),
		alignItems("flex-start"),
	)
	rule(".rpta-yaxis",
		display("flex"),
		flexDirection("column"),
		justifyContent("space-between"),
		height("120px"),
		flexShrink("0"),
		fontSize("0.62rem"),
		textAlign("right"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rpta-yaxis-wd",
		height("56px"),
	)
	rule(".rpta-chart-plot",
		flex("1"),
		minWidth("0"),
	)
	// Histogram scale caption — states what full bar width means.
	rule(".rpta-hist-scale",
		fontSize("0.68rem"),
		color("var(--text-faint)"),
		textAlign("right"),
		fontStyle("italic"),
	)
	// Subhead direction glyphs (↗ in, ↘ out, ⚠ watch).
	rule(".rpta-sub-glyph",
		fontWeight("700"),
	)
	// Adherence month header row.
	rule(".rpta-bud-mon",
		height("auto"),
		fontSize("0.62rem"),
		fontWeight("700"),
		textAlign("center"),
		color("var(--text-faint)"),
		background("transparent"),
	)
	// Masthead sparkline caption.
	rule(".rpta-fig-spark-cap",
		display("block"),
		fontSize("0.62rem"),
		color("var(--text-faint)"),
	)
	// Quiet source-page links ("Net worth →") beside metrics.
	rule(".rpta-src",
		fontSize("0.72rem"),
		color("var(--text-faint)"),
		textDecoration("none"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-src:hover",
		color("var(--accent)"),
		textDecoration("underline"),
	)
	rule(".rpta-srcrow",
		display("flex"),
		justifyContent("flex-end"),
	)
	// Spending-by-tag rows (§05): the tag reads as the chip it is elsewhere in
	// the app; the delta column tones like the category table (spend up = red).
	rule(".rpta-tag-chip",
		display("inline-block"),
		padding("0.05rem 0.5rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		fontSize("0.8rem"),
	)
	rule(".rpta-tag-delta",
		fontSize("0.75rem"),
		minWidth("3.4rem"),
		textAlign("right"),
		flexShrink("0"),
	)
	rule(".rpta-tag-note",
		fontSize("0.75rem"),
	)
	// Win chips lead with a check glyph.
	rule(".rpta-win-check",
		color("var(--up, #4ea777)"),
		fontWeight("700"),
		marginRight("0.35rem"),
	)
	// Masthead net-worth sparkline sits quietly under the figure.
	rule(".rpta-fig-spark",
		marginTop("0.35rem"),
		opacity("0.8"),
	)
	// Kept-% inline meter in the monthly review table.
	rule(".rpta-td-kept",
		minWidth("6.5rem"),
	)
	rule(".rpta-kept-meter",
		height("3px"),
		marginTop("0.25rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--border) 60%, transparent)"),
		overflow("hidden"),
	)
	rule(".rpta-kept-fill",
		height("100%"),
		borderRadius("999px"),
		background("var(--up, #4ea777)"),
	)
	rule(".rpta-kept-red",
		background("var(--down, #d8716f)"),
	)
	// Spending-by-weekday mini bars.
	rule(".rpta-weekday",
		marginTop("0.9rem"),
	)
	rule(".rpta-wd-bars",
		display("flex"),
		alignItems("flex-end"),
		gap("0.6rem"),
		maxWidth("22rem"),
	)
	rule(".rpta-wd-col",
		flex("1"),
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap("0.25rem"),
	)
	rule(".rpta-wd-track",
		width("100%"),
		height("56px"),
		display("flex"),
		alignItems("flex-end"),
	)
	rule(".rpta-wd-fill",
		width("100%"),
		borderRadius("3px 3px 0 0"),
		background("color-mix(in srgb, var(--accent) 45%, transparent)"),
	)
	rule(".rpta-wd-peak",
		background("var(--accent)"),
	)
	rule(".rpta-wd-day",
		fontSize("0.66rem"),
		color("var(--text-faint)"),
	)
	// Category dots reuse the flow-dot circle at a smaller row scale, inline
	// with the name.
	rule(".rpta-cat-dot",
		width("8px"),
		height("8px"),
		marginRight("0.45rem"),
	)
	rule(".rpta-cat-title",
		display("inline-flex"),
		alignItems("center"),
	)
	// Section-header actions: existing controls + the ask-the-assistant button.
	rule(".rpta-sec-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexShrink("0"),
	)
	rule(".rpta-ask",
		display("inline-flex"),
		alignItems("center"),
		padding("0.3rem 0.7rem"),
		border("1px solid color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
		borderRadius("999px"),
		color("var(--text)"),
		fontSize("0.78rem"),
		fontFamily("inherit"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("background 0.12s ease"),
	)
	rule(".rpta-ask:hover",
		background("color-mix(in srgb, var(--accent) 18%, transparent)"),
	)
	// Year-spend histograms (§04 categories, §05 tags).
	rule(".rpta-hist",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		marginTop("0.6rem"),
	)
	rule(".rpta-hist-row",
		display("grid"),
		prop("grid-template-columns", "minmax(9rem, 13rem) 1fr 7rem 4.5rem 5.5rem"),
		alignItems("center"),
		gap("0.75rem"),
		padding("0.22rem 0.3rem"),
		border("none"),
		background("transparent"),
		fontFamily("inherit"),
		fontSize("0.85rem"),
		color("var(--text)"),
		textAlign("left"),
		borderRadius("6px"),
	)
	rule(".rpta-hist-btn",
		cursor("pointer"),
	)
	rule(".rpta-hist-btn:hover",
		background("var(--hover)"),
	)
	rule(".rpta-hist-label",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-hist-track",
		height("14px"),
		borderRadius("4px"),
		background("color-mix(in srgb, var(--border) 45%, transparent)"),
		overflow("hidden"),
	)
	rule(".rpta-hist-fill",
		height("100%"),
		borderRadius("4px"),
		prop("opacity", "0.85"),
	)
	rule(".rpta-hist-amt",
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rpta-hist-delta",
		fontSize("0.75rem"),
		textAlign("right"),
	)
	rule(".rpta-hist-meta",
		fontSize("0.75rem"),
		textAlign("right"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-hist-row .rpta-tag-chip",
		prop("justify-self", "start"),
		maxWidth("100%"),
	)
	// Goal progress rows (§06): a coverage track with a solid saved band.
	rule(".rpta-goal-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.7rem"),
		marginTop("0.6rem"),
	)
	rule(".rpta-goal-top",
		display("flex"),
		alignItems("baseline"),
		gap("0.6rem"),
		flexWrap("wrap"),
	)
	rule(".rpta-goal-name",
		fontWeight("600"),
	)
	rule(".rpta-goal-fig",
		marginLeft("auto"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rpta-goal-chip",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		padding("0.1rem 0.45rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
	)
	rule(".rpta-chip-up",
		color("var(--up, #4ea777)"),
		borderColor("color-mix(in srgb, var(--up, #4ea777) 50%, var(--border))"),
	)
	rule(".rpta-chip-down",
		color("var(--down, #d8716f)"),
		borderColor("color-mix(in srgb, var(--down, #d8716f) 50%, var(--border))"),
	)
	rule(".rpta-chip-warn",
		color("var(--warn, #d8a24a)"),
		borderColor("color-mix(in srgb, var(--warn, #d8a24a) 50%, var(--border))"),
	)
	rule(".rpta-chip-dim",
		color("var(--text-dim)"),
	)
	rule(".rpta-goal-track",
		position("relative"),
		height("8px"),
		marginTop("0.35rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--border) 45%, transparent)"),
		overflow("hidden"),
	)
	rule(".rpta-goal-cov",
		position("absolute"),
		inset("0 auto 0 0"),
		prop("opacity", "0.35"),
		borderRadius("999px"),
	)
	rule(".rpta-goal-saved",
		position("absolute"),
		inset("0 auto 0 0"),
		borderRadius("999px"),
	)
	// Budget adherence strips (§07): twelve month cells per budget.
	rule(".rpta-bud-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.45rem"),
		marginTop("0.6rem"),
	)
	rule(".rpta-bud-row",
		display("grid"),
		prop("grid-template-columns", "minmax(9rem, 16rem) 1fr minmax(9rem, auto)"),
		alignItems("center"),
		gap("0.75rem"),
		fontSize("0.85rem"),
	)
	rule(".rpta-bud-cells",
		display("flex"),
		gap("3px"),
	)
	rule(".rpta-bud-cell",
		flex("1"),
		maxWidth("2.2rem"),
		height("14px"),
		borderRadius("3px"),
	)
	rule(".rpta-bud-quiet",
		background("color-mix(in srgb, var(--border) 45%, transparent)"),
	)
	rule(".rpta-bud-under",
		background("color-mix(in srgb, var(--up, #4ea777) 65%, transparent)"),
	)
	rule(".rpta-bud-near",
		background("color-mix(in srgb, var(--warn, #d8a24a) 75%, transparent)"),
	)
	rule(".rpta-bud-over",
		background("var(--down, #d8716f)"),
	)
	rule(".rpta-bud-verdict",
		fontSize("0.75rem"),
		textAlign("right"),
		whiteSpace("nowrap"),
	)
	// Fee/interest rows in the problem-spots section.
	rule(".rpta-cost-kind",
		fontSize("0.62rem"),
		fontWeight("700"),
		letterSpacing("0.07em"),
		padding("0.1rem 0.4rem"),
		borderRadius("4px"),
		background("color-mix(in srgb, var(--down, #d8716f) 14%, transparent)"),
		color("var(--down, #d8716f)"),
		marginRight("0.5rem"),
		flexShrink("0"),
	)
	// Interest-drag bar under the debt table's yearly-interest figures.
	rule(".rpta-bar-red",
		height("3px"),
		marginTop("0.25rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--border) 60%, transparent)"),
		overflow("hidden"),
	)
	rule(".rpta-bar-red-fill",
		height("100%"),
		borderRadius("999px"),
		background("var(--down, #d8716f)"),
	)
	rule(".rpta-flow-side",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		maxWidth("34rem"),
	)

	// ── Dense document tables ────────────────────────────────────────────────
	rule(".rpta-table",
		width("100%"),
		prop("border-collapse", "collapse"),
		fontSize("0.85rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rpta-table th",
		textAlign("right"),
		padding("0.35rem 0.6rem"),
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rpta-table th:first-child",
		textAlign("left"),
		paddingLeft("0"),
	)
	rule(".rpta-table td",
		padding("0.4rem 0.6rem"),
		borderBottom("1px solid var(--border-subtle, var(--border))"),
	)
	rule(".rpta-td-name",
		textAlign("left"),
		paddingLeft("0"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".rpta-td-num",
		textAlign("right"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-td-strong",
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".rpta-tr-red td",
		background("color-mix(in srgb, var(--down, #d8716f) 6%, transparent)"),
	)
	rule(".rpta-tr-kept td",
		borderTop("1px solid var(--border)"),
		fontWeight("700"),
		color("var(--up, #4ea777)"),
	)

	// ── Year-in-motion charts ────────────────────────────────────────────────
	rule(".rpta-charts3",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(260px, 1fr))"),
		gap("1.25rem"),
		marginTop("0.5rem"),
	)
	rule(".rpta-chart",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		minWidth("0"),
	)

	// ── Category review rows ─────────────────────────────────────────────────
	rule(".rpta-narrative",
		margin("0"),
		fontSize("1.08rem"),
		lineHeight("1.5"),
		color("var(--text)"),
		maxWidth("52rem"),
	)
	rawBlock(`.rpta-cat-head,.rpta-cat-row{display:grid;grid-template-columns:minmax(160px,2.2fr) minmax(90px,auto) minmax(80px,auto) minmax(130px,1fr) minmax(84px,auto) 3.4rem;gap:0.9rem;align-items:center}`)
	rule(".rpta-cat-head",
		padding("0.3rem 0"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rpta-cat-h, .rpta-cat-h-name",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		textAlign("right"),
	)
	rule(".rpta-cat-h-name", textAlign("left"))
	rule(".rpta-cat-h-spark", textAlign("center"))
	rule(".rpta-cat-rows",
		display("flex"),
		flexDirection("column"),
	)
	rule(".rpta-cat-row",
		padding("0.45rem 0"),
		borderBottom("1px solid var(--border-subtle, var(--border))"),
	)
	rule(".rpta-cat-row:hover",
		background("var(--hover)"),
	)
	rule(".rpta-cat-name",
		prop("appearance", "none"),
		display("flex"),
		flexDirection("column"),
		alignItems("stretch"),
		gap("0.3rem"),
		background("transparent"),
		border("0"),
		padding("0"),
		font("inherit"),
		color("var(--text)"),
		fontWeight("600"),
		textAlign("left"),
		cursor("pointer"),
		minWidth("0"),
	)
	rule(".rpta-cat-name:hover span:first-child",
		textDecoration("underline"),
		prop("text-decoration-style", "dotted"),
		prop("text-underline-offset", "3px"),
	)
	rule(".rpta-cat-amt",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		textAlign("right"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-cat-avg, .rpta-cat-share",
		fontVariantNumeric("tabular-nums"),
		textAlign("right"),
		fontSize("0.85rem"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-cat-spark",
		display("flex"),
		justifyContent("center"),
		color("var(--text-dim)"),
	)
	rule(".rpta-cat-delta",
		fontSize("0.82rem"),
		fontVariantNumeric("tabular-nums"),
		textAlign("right"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-zeroed summary",
		cursor("pointer"),
		color("var(--text-dim)"),
		fontSize("0.85rem"),
		padding("0.5rem 0"),
	)

	// ── Where-it-goes two-column layout ─────────────────────────────────────
	rule(".rpta-cols2",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(320px, 1fr))"),
		gap("1.5rem"),
	)
	rule(".rpta-col",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
		minWidth("0"),
	)

	// ── Watch list rows ──────────────────────────────────────────────────────
	rule(".rpta-watch-row",
		display("flex"),
		alignItems("center"),
		gap("0.9rem"),
		padding("0.4rem 0"),
		borderBottom("1px solid var(--border-subtle, var(--border))"),
	)
	rule(".rpta-watch-name",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".rpta-watch-delta",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		whiteSpace("nowrap"),
	)
	rule(".rpta-watch-amt",
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		color("var(--text)"),
	)

	// ── Problem lines ────────────────────────────────────────────────────────
	rule(".rpta-prob-line",
		margin("0"),
		fontSize("0.95rem"),
		color("var(--text)"),
	)

	// ── The plan ─────────────────────────────────────────────────────────────
	rule(".rpta-plan",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	rule(".rpta-plan-item",
		display("flex"),
		alignItems("flex-start"),
		gap("1rem"),
		padding("0.9rem 1rem"),
		border("1px solid color-mix(in srgb, var(--accent) 25%, var(--border))"),
		borderRadius("12px"),
		background("color-mix(in srgb, var(--accent) 4%, transparent)"),
	)
	rule(".rpta-plan-n",
		fontSize("1.4rem"),
		lineHeight("1.1"),
		fontWeight("600"),
		color("var(--accent)"),
		fontVariantNumeric("tabular-nums"),
		flexShrink("0"),
	)
	rule(".rpta-plan-body",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		minWidth("0"),
	)
	rule(".rpta-plan-action",
		fontWeight("600"),
		color("var(--text)"),
		fontSize("1.02rem"),
	)
	rule(".rpta-plan-detail",
		fontSize("0.88rem"),
		color("var(--text-dim)"),
	)
	rule(".rpta-plan-link",
		fontSize("0.82rem"),
		color("var(--accent)"),
		textDecoration("none"),
		fontWeight("600"),
	)
	rule(".rpta-plan-link:hover", textDecoration("underline"))

	// #56: a masthead figure's value is a button that opens its provenance
	// popover — the button must look exactly like the former span, with a
	// quiet dotted underline on hover as the only affordance.
	rule(".rpta-fig-btn",
		prop("background", "none"),
		prop("border", "0"),
		prop("padding", "0"),
		prop("font", "inherit"),
		prop("color", "inherit"),
		prop("text-align", "left"),
		prop("cursor", "pointer"),
	)
	rule(".rpta-fig-btn:hover, .rpta-fig-btn:focus-visible",
		prop("text-decoration", "underline dotted"),
		prop("text-underline-offset", "0.35em"),
	)
	// No display here — .hidden-menu's display:none must keep winning when the
	// popover is closed (same single-class specificity, later rule would win).
	rule(".rpta-prov-pop",
		prop("min-width", "17rem"),
		prop("max-width", "23rem"),
		prop("padding", "0.65rem 0.8rem"),
		prop("gap", "0.35rem"),
		prop("font-size", "0.8rem"),
		prop("white-space", "normal"),
		prop("font-family", "inherit"),
	)
	rule(".rpta-prov-title",
		prop("font-weight", "600"),
	)
	rule(".rpta-prov-line",
		prop("color", "var(--text-dim)"),
		prop("line-height", "1.45"),
	)

	// Print: flatten stickiness so Save-as-PDF captures the whole document.
	rawBlock(`@media print{.rpta-index{position:static;backdrop-filter:none}.rpta-sec{break-inside:avoid-page}}`)
}
