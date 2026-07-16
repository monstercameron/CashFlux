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

	// ── Sections: the verdict spine ──────────────────────────────────────────
	rule(".rpta-sec",
		borderLeft("3px solid var(--rpta-zone)"),
		paddingLeft("1.4rem"),
		prop("scroll-margin-top", "6.5rem"),
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

	// ── Money flow: sankey + per-$100 companion ──────────────────────────────
	rule(".rpta-sankey",
		width("100%"),
		overflowX("auto"),
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

	// Print: flatten stickiness so Save-as-PDF captures the whole document.
	rawBlock(`@media print{.rpta-index{position:static;backdrop-filter:none}.rpta-sec{break-inside:avoid-page}}`)
}
