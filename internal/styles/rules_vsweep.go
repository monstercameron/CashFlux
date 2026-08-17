// SPDX-License-Identifier: MIT

package styles

// registerVSweep holds the CSS for the 2026-07-03 world-class visual/UX sweep's
// remaining tickets (C340–C362). Registered last so these overrides win over the
// base rules they refine.
func registerVSweep() {
	// ── C342/C343: name the window over the hero's stat row ─────────────────
	// The four figures (income / spending / net / savings rate) cover the
	// SELECTED period, not "this month" — and the savings rate among them is the
	// one most often read against /health's three-month average. The caption
	// carries the spacing the row used to own so the two read as one block.
	rule(".home-hero-stats-block",
		marginTop("1rem"),
		paddingTop("0.85rem"),
	)
	rule(".home-hero-stats-block .home-hero-stats",
		marginTop("0"),
		paddingTop("0.35rem"),
	)
	// The always-on period chip reads quieter than the "Today" exception chip it
	// shares a slot with: one is context, the other is a warning that a figure
	// ignores the picker.
	rule(".w-today.w-window",
		borderStyle("dashed"),
		textTransform("none"),
		letterSpacing("0.02em"),
	)

	// ── C353: the real rate sits beside the abstract score ──────────────────
	rule(".alloc-dest-chip-real",
		marginLeft("0.3rem"),
		fontVariantNumeric("tabular-nums"),
	)

	// ── C348: /subscriptions keeps its actions until you reach for them ─────
	// A list of twenty rows carrying two resting buttons each is forty controls
	// competing with the twenty numbers the page is actually about. "Remind me"
	// is a rare action; it fades in on hover or keyboard focus, the way the
	// transactions ledger already does it. It stays visible whenever the row is
	// focused within, so it is never keyboard-unreachable, and stays visible
	// unconditionally on touch, where there is no hover to reveal it on.
	rule(".sub-row .sub-actions .btn-reveal",
		opacity("0"),
		transition("opacity var(--motion-quick, 120ms) ease"),
	)
	rule(".sub-row:hover .sub-actions .btn-reveal, .sub-row:focus-within .sub-actions .btn-reveal",
		opacity("1"),
	)
	ruleMedia("(hover: none)", ".sub-row .sub-actions .btn-reveal",
		opacity("1"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".sub-row .sub-actions .btn-reveal",
		transition("none"),
	)

	// Cross-links between the page's three views of the same subscriptions.
	rule(".subs-xlink",
		marginTop("0.35rem"),
		fontSize("var(--type-12)"),
	)

	// ── C357: the rules list IS the ordering surface ────────────────────────
	// The precedence number moved onto the row when the duplicate "Rule order"
	// section was retired. Quiet and tabular so it reads as an index, not a
	// figure competing with the match count on the right.
	rule(".rule-prec",
		flex("none"),
		minWidth("1.25rem"),
		fontVariantNumeric("tabular-nums"),
		fontSize("var(--type-12)"),
		textAlign("right"),
	)

	// ── C358: the plan's arc, quiet under its ending figure ─────────────────
	rule(".plan-scenario-arc",
		display("block"),
		fontSize("var(--type-12)"),
		marginTop("0.1rem"),
	)

	// ── C360: the category map is navigation, so its chips look reachable ───
	rule("a.cat-map-chip, a.cat-map-sub",
		textDecoration("none"),
		color("inherit"),
		cursor("pointer"),
	)
	rule("a.cat-map-chip:hover, a.cat-map-sub:hover, a.cat-map-chip:focus-visible, a.cat-map-sub:focus-visible",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)

	// ── C379: the target tick on a drift bar ────────────────────────────────
	// The bar's LENGTH is the current weight and the tick marks the target, so
	// "on target" reads as a full-looking bar with the tick at its end rather
	// than as an empty bar — the same value-on-a-scale reading the health
	// factors use.
	rule(".inv-alloc-track",
		position("relative"),
	)
	rule(".inv-drift-tick",
		position("absolute"),
		top("-2px"),
		bottom("-2px"),
		width("2px"),
		background("var(--text)"),
		opacity("0.55"),
	)
	rule(".inv-drift-over",
		color("var(--warn, var(--text))"),
	)
	rule(".inv-drift-under",
		color("var(--text-dim)"),
	)

	rule(".home-hero-stats-window",
		margin("0"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		fontSize("0.66rem"),
	)
	// ─── C402: to-do bulk selection ──────────────────────────────────────────
	// The bar sits above the list and stays put while you scroll a long page —
	// a selection you built over three screenfuls with the action bar scrolled
	// off the top is a selection you cannot act on.
	rule(".todo-bulk-entry",
		display("flex"),
		justifyContent("flex-end"),
		marginBottom("0.5rem"),
	)
	rule(".todo-bulkbar",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
		position("sticky"),
		top("0"),
		zIndex("3"),
		padding("0.5rem 0.65rem"),
		marginBottom("0.6rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface-2, var(--surface))"),
	)
	rule(".todo-select",
		flex("0 0 auto"),
		width("1rem"),
		height("1rem"),
		marginRight("0.15rem"),
		accentColor("var(--accent)"),
		cursor("pointer"),
	)

	// ─── C405: the ready-made automation gallery ─────────────────────────────
	rule(".wf-preset-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(15rem, 1fr))"),
		gap("0.75rem"),
	)
	rule(".wf-preset-card",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.4rem"),
		padding("0.85rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface)"),
	)
	rule(".wf-preset-name",
		margin("0"),
		fontSize("0.95rem"),
		lineHeight("1.3"),
	)
	rule(".wf-preset-desc",
		margin("0"),
		flex("1 1 auto"),
		fontSize("0.82rem"),
		lineHeight("1.45"),
		color("var(--text-dim)"),
	)
	rule(".wf-preset-meta",
		display("flex"),
		flexWrap("wrap"),
		gap("0.35rem"),
	)

}
