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

	rule(".home-hero-stats-window",
		margin("0"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		fontSize("0.66rem"),
	)
}
