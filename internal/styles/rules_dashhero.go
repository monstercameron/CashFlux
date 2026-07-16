// SPDX-License-Identifier: MIT

package styles

// registerDashHeroSurface tightens the Dashboard hero's LAYOUT after an aggressive
// spacing/hierarchy audit. Registered after registerGenerated() so these overrides win
// over the base .home-hero rules. Three problems addressed, all spatial:
//   - the net-worth figure sat too low under an oversized greeting;
//   - the 6-month sparkline was capped at 360px and pushed to the right edge by
//     space-between, stranding a large dead gap in the middle of the hero;
//   - the disabled "Add a daily quote" opt-in occupied a full bordered footer band —
//     prime real estate on the most-viewed page for a low-value control.
func registerDashHeroSurface() {
	// The greeting is a warm touch, not the headline. Shrink it and tighten the gap so
	// the net-worth figure — the number the user opened the app for — sits higher.
	rule(".home-hero-greeting",
		fontSize("1.4rem"),
	)
	rule(".home-hero-top",
		marginBottom("0.5rem"),
	)

	// Let the sparkline READ as a living headline that fills the row beside the number
	// instead of a fixed chip stranded at the right with a dead centre. flex-start +
	// flex:1 auto makes the chart grow from the figure to the card edge — no void.
	rule(".home-hero-main",
		justifyContent("flex-start"),
		gap("2.25rem"),
	)
	rule(".home-hero-spark",
		flex("1 1 auto"),
		maxWidth("none"),
		minWidth("180px"),
	)

	// Tighten the vertical stack so the hero stops pushing "Needs attention" (the
	// actionable content) down the page.
	rule(".home-hero-stats",
		marginTop("1rem"),
		paddingTop("0.85rem"),
	)
	rule(".home-hero-actions",
		marginTop("0.9rem"),
	)

	// Demote the disabled daily-quote opt-in: drop the divider band and the top padding
	// so it reads as a quiet appendix under the actions, not a full section of its own.
	rule(".home-hero-quote--off",
		marginTop("0.65rem"),
		paddingTop("0"),
		borderTop("0"),
	)

	// --- Accounts summary: kill the "dead middle" -----------------------------------
	// The net-worth summary was a 1.6fr/1fr grid where the hero stat spanned BOTH rows
	// (grid-row 1/3) but held one centred figure, so the tall left card was ~half empty
	// next to the stacked Assets/Liabilities. Re-lay it as one balanced 3-column row
	// (Net worth | Assets | Liabilities) — the same pattern Budgets/Goals use — so every
	// cell sizes to content and no space is wasted. Net worth stays dominant via the
	// wider column + the hero font size.
	rule(".nw-summary",
		gridTemplateColumns("1.4fr 1fr 1fr"),
		gridAutoRows("auto"),
	)
	rule(".nw-summary .stat-hero",
		gridRow("auto"),
		justifyContent("flex-start"),
	)

	// First-paint skeleton band: while the bento grid is deferred (dashReady), a
	// full-width 4-column sub-grid of shimmer placeholder tiles reserves the grid's
	// space so the dashboard reads as loading instead of flashing empty and then
	// popping the real tiles in ~300ms later.
	rule(".dash-skeleton",
		gridColumn("1 / -1"),
		display("grid"),
		gridTemplateColumns("repeat(4, 1fr)"),
		gap("0.75rem"),
	)
	rule(".dash-skel-tile",
		minHeight("128px"),
		borderRadius("var(--radius)"),
		background("linear-gradient(100deg, var(--bg-card) 30%, color-mix(in srgb, var(--text) 6%, var(--bg-card)) 50%, var(--bg-card) 70%)"),
		backgroundSize("200% 100%"),
		animation("dashSkelShimmer 1.4s ease-in-out infinite"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".dash-skel-tile",
		animation("none"),
	)
	keyframes("dashSkelShimmer",
		at("from", backgroundPosition("200% 0")),
		at("to", backgroundPosition("-200% 0")),
	)
}
