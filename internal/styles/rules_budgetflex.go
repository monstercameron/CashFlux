// SPDX-License-Identifier: MIT

package styles

// registerBudgetFlexSurface emits the /budgets flex-methodology view (BG2): the
// "one number for day-to-day" surface. It is authored as a single calm instrument
// rather than a form — a large serif figure of what's still spendable, a wide
// horizon meter whose pace tick marks where the calendar sits (so a fill past the
// tick reads instantly as "spending faster than the month"), and two quiet ledgers
// for fixed commitments and non-monthly set-asides. It borrows the surface's existing
// vocabulary: the Fraunces hero numeral, uppercase micro-labels, tabular figures, and
// the accent / warn / down tones. Registered after the generated sheet so equal-
// specificity refinements win.
func registerBudgetFlexSurface() {
	// The flex methodology IS the whole budgets view, not one tile in a 4-up grid.
	// Unlike the other budget tiles it renders as a bare .budgets-flex grid child
	// (no uiw.Widget shell, so no data-widget wrapper), so target it directly: span
	// every column and, with order:3, sit right AFTER the method toolbar (order:2)
	// instead of ahead of it (the default order:0 put it above the picker).
	rule(".bento-budgets > .budgets-flex",
		prop("grid-column", "1 / -1"),
		order("3"),
	)

	// --- the card ---------------------------------------------------------------
	// Mirror the sibling .w tile shell: theme surface tokens and the theme corner
	// radius (var(--radius)) rather than a hardcoded radius, so the surface follows
	// the theming engine and sits flush with the square method-toolbar tile beside it.
	rule(".budgets-flex",
		display("flex"),
		flexDirection("column"),
		gap("1.1rem"),
		padding("1.4rem 1.5rem 1.5rem"),
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
	)

	// --- hero band --------------------------------------------------------------
	rule(".bflex-hero",
		display("flex"),
		flexDirection("column"),
		gap("1.15rem"),
	)
	// Eyebrow: the kicker on the left, the classify action pinned right.
	rule(".bflex-eyebrow",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
	)
	rule(".bflex-kicker",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		minWidth("0"),
	)
	rule(".bflex-kicker-label",
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.09em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".bflex-kicker-dot",
		width("3px"),
		height("3px"),
		borderRadius("var(--radius-pill)"),
		background("var(--text-faint)"),
		opacity("0.55"),
	)
	rule(".bflex-kicker-period",
		fontSize("0.72rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
	)
	rule(".bflex-classify", flexShrink("0"))

	// --- the live reading: serif figure + word ----------------------------------
	rule(".bflex-reading",
		display("flex"),
		flexDirection("column"),
		gap("0.85rem"),
	)
	rule(".bflex-figure",
		display("flex"),
		alignItems("baseline"),
		gap("0.55rem"),
		flexWrap("wrap"),
	)
	// The signature numeral: Fraunces, sized as the one thing the eye lands on.
	rule(".bflex-num",
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
		fontSize("clamp(2.6rem, 6vw, 3.6rem)"),
		fontWeight("600"),
		letterSpacing("-0.02em"),
		lineHeight("1"),
		prop("font-variant-numeric", "tabular-nums"),
		color("var(--text)"),
	)
	rule(".bflex-word",
		fontSize("1rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
	)
	// Over budget flips both figure and word into the danger tone.
	rule(".bflex-figure.is-over .bflex-num", color("var(--down, #d8716f)"))
	rule(".bflex-figure.is-over .bflex-word", color("var(--down, #d8716f)"))

	// --- signature horizon meter ------------------------------------------------
	rule(".bflex-meter",
		position("relative"),
		height("14px"),
		margin("0.1rem 0"),
	)
	rule(".bflex-meter-track",
		position("absolute"),
		inset("0"),
		borderRadius("var(--radius)"),
		overflow("hidden"),
		background("color-mix(in srgb, var(--text-faint) 18%, transparent)"),
	)
	rule(".bflex-meter-fill",
		height("100%"),
		borderRadius("var(--radius)"),
		transition("width var(--wonder-dur-slow, 300ms) var(--wonder-ease-out)"),
	)
	rule(".bflex-meter-fill.is-empty", background("transparent"))
	rule(".bflex-meter-fill.is-ok",
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 78%, transparent), var(--accent))"),
	)
	rule(".bflex-meter-fill.is-warn",
		background("linear-gradient(90deg, color-mix(in srgb, var(--warn, #cfa14e) 78%, transparent), var(--warn, #cfa14e))"),
	)
	rule(".bflex-meter-fill.is-over",
		background("linear-gradient(90deg, color-mix(in srgb, var(--down, #d8716f) 78%, transparent), var(--down, #d8716f))"),
	)
	// The pace tick: where the calendar sits in the period. It protrudes past the
	// rail with a card-colored ring so it reads as a threshold, not a slice.
	rule(".bflex-meter-pace",
		position("absolute"),
		top("-3px"),
		bottom("-3px"),
		width("2px"),
		marginLeft("-1px"),
		borderRadius("1px"),
		background("var(--text)"),
		prop("box-shadow", "0 0 0 2px var(--bg-card)"),
	)

	// --- context line: spent-of-target · pace verdict · days left · edit ---------
	rule(".bflex-context",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem 0.9rem"),
	)
	rule(".bflex-spent",
		fontSize("0.9rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".bflex-context-right",
		display("flex"),
		alignItems("center"),
		gap("0.7rem"),
		flexWrap("wrap"),
	)
	// The pace verdict pill: the one-word read, tinted by state.
	rule(".bflex-pace",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		padding("0.2rem 0.55rem 0.2rem 0.45rem"),
		borderRadius("var(--radius-sm)"),
		fontSize("0.75rem"),
		fontWeight("600"),
	)
	rule(".bflex-pace-dot",
		width("0.5rem"),
		height("0.5rem"),
		borderRadius("var(--radius-pill)"),
		flexShrink("0"),
	)
	rule(".bflex-pace.is-ok",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".bflex-pace.is-ok .bflex-pace-dot", background("var(--accent)"))
	rule(".bflex-pace.is-warn",
		color("color-mix(in srgb, var(--warn, #cfa14e) 72%, var(--text))"),
		background("color-mix(in srgb, var(--warn, #cfa14e) 15%, transparent)"),
	)
	rule(".bflex-pace.is-warn .bflex-pace-dot", background("var(--warn, #cfa14e)"))
	rule(".bflex-pace.is-over",
		color("var(--down, #d8716f)"),
		background("color-mix(in srgb, var(--down, #d8716f) 13%, transparent)"),
	)
	rule(".bflex-pace.is-over .bflex-pace-dot", background("var(--down, #d8716f)"))
	rule(".bflex-days",
		fontSize("0.78rem"),
		color("var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".bflex-edit", flexShrink("0"))

	// --- empty state: an invitation, not a dead "$0 of $0" ----------------------
	rule(".bflex-empty",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1.5rem"),
		flexWrap("wrap"),
		padding("0.5rem 0 0.4rem"),
	)
	rule(".bflex-empty-copy",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		maxWidth("34rem"),
	)
	rule(".bflex-empty-title",
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
		fontSize("1.5rem"),
		fontWeight("600"),
		letterSpacing("-0.01em"),
		lineHeight("1.15"),
		color("var(--text)"),
		margin("0"),
	)
	rule(".bflex-empty-body",
		fontSize("0.9rem"),
		lineHeight("1.5"),
		color("var(--text-dim)"),
		margin("0"),
	)
	rule(".bflex-empty-cta", flexShrink("0"))

	// --- inline editor ----------------------------------------------------------
	rule(".bflex-editor",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		padding("0.2rem 0"),
	)
	rule(".bflex-editor-label",
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".bflex-editor-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
	)
	rule(".bflex-editor-input",
		maxWidth("14rem"),
		fontSize("1.1rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)

	// --- the two ledgers: fixed commitments + non-monthly set-asides ------------
	rule(".bflex-ledgers",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		gap("0.9rem"),
	)
	rule(".bflex-ledger",
		display("flex"),
		flexDirection("column"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		padding("0.9rem 1rem 1rem"),
	)
	rule(".bflex-ledger-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.6rem"),
		paddingBottom("0.55rem"),
		marginBottom("0.3rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".bflex-ledger-title",
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.07em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		margin("0"),
	)
	rule(".bflex-count",
		fontSize("0.72rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".bflex-count.is-done", color("var(--accent)"))
	rule(".bflex-list",
		display("flex"),
		flexDirection("column"),
	)
	rule(".bflex-ledger-empty",
		fontSize("0.82rem"),
		lineHeight("1.45"),
		color("var(--text-dim)"),
		margin("0.35rem 0 0.15rem"),
	)

	// One ledger row.
	rule(".bflex-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		padding("0.5rem 0"),
		borderTop("1px solid var(--border-subtle)"),
	)
	rule(".bflex-list .bflex-row:first-child", borderTop("0"))
	rule(".bflex-row-main",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		minWidth("0"),
	)
	rule(".bflex-tick", color("var(--text-faint)"))
	rule(".bflex-row.is-paid .bflex-tick", color("var(--up, #54b884)"))
	rule(".bflex-row-name",
		fontSize("0.9rem"),
		fontWeight("500"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".bflex-row-side",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		flexShrink("0"),
	)
	rule(".bflex-row-side.is-stacked",
		flexDirection("column"),
		alignItems("flex-end"),
		gap("0.1rem"),
	)
	rule(".bflex-row-fig",
		fontSize("0.85rem"),
		fontWeight("600"),
		color("var(--text)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".bflex-row-sub",
		fontSize("0.75rem"),
		color("var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// Paid/unpaid badge.
	rule(".bflex-badge",
		fontSize("0.64rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		padding("0.1rem 0.4rem"),
		borderRadius("var(--radius-sm)"),
		background("color-mix(in srgb, var(--text-faint) 12%, transparent)"),
		whiteSpace("nowrap"),
	)
	rule(".bflex-badge.is-paid",
		color("var(--up, #54b884)"),
		background("color-mix(in srgb, var(--up, #54b884) 15%, transparent)"),
	)

	// --- responsive + reduced-motion --------------------------------------------
	// Stack the two ledgers on narrow viewports.
	ruleMedia("(max-width:720px)", ".bflex-ledgers", gridTemplateColumns("1fr"))
	// The empty-state CTA drops below its copy when there's no room beside it.
	ruleMedia("(max-width:640px)", ".bflex-empty",
		flexDirection("column"),
		alignItems("flex-start"),
	)
	// Honor reduced-motion: no meter fill animation.
	ruleMedia("(prefers-reduced-motion:reduce)", ".bflex-meter-fill", transition("none"))
}
