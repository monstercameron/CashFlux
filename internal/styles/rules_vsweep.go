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

	rule(".home-hero-stats-window",
		margin("0"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		fontSize("0.66rem"),
	)
}
