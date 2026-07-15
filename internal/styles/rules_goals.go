// SPDX-License-Identifier: MIT

package styles

// registerGoalsSurface emits the /goals redesign chrome: full-width goal cards,
// the optional (toggle-on) contribution planner, the standards-based round-ups
// modal, and the more-visual earmarks views. Registered AFTER registerGenerated()
// (see Register()), so same-selector rules here OVERRIDE the generated defaults —
// the goals redesign lives entirely in this file, never in rules_gen.go, so it
// stays conflict-free while other surfaces are edited in parallel. All colours use
// the theme tokens (var(--text)/(--border)/(--bg-card)/(--bg-elev)/--accent); never
// var(--fg)/(--line)/(--dim)/(--faint) (those are undefined and render dark).
func registerGoalsSurface() {
	// Design tokens reused below (theme-safe): a serif display face for hero numerals,
	// a faint hairline, and a subtle progress track.
	const (
		serif = "var(--font-display), Fraunces, Georgia, serif"
		hair  = "1px solid color-mix(in srgb, var(--border) 60%, transparent)"
		track = "color-mix(in srgb, var(--text) 9%, transparent)"
	)

	// --- Task 1: FULL-WIDTH goal cards -------------------------------------------
	// One card per row (was a 2-col auto-fill grid). Full width lets each card breathe
	// and lay its figures out as a scannable stat row.
	rule(".bento-goals .goal-list",
		display("grid"),
		gridTemplateColumns("1fr"),
		gap("0.9rem"),
		alignItems("stretch"),
	)
	rule(".bento-goals .goal-card",
		position("relative"),
		display("flex"),
		flexDirection("column"),
		minHeight("auto"),
		padding("1.25rem 1.5rem 1.15rem"),
		border("1px solid var(--border)"),
		borderRadius("16px"),
		background("color-mix(in srgb, var(--bg-elev) 55%, transparent)"),
		boxShadow("inset 4px 0 0 var(--accent)"),
		transition("border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease"),
	)
	rule(".bento-goals .goal-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 30%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 80%, transparent)"),
		transform("none"),
	)
	// Header: goal name as a serif display line, badges trailing.
	rule(".bento-goals .goal-card-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginBottom("0.35rem"),
	)
	rule(".bento-goals .goal-card-title",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontFamily(serif),
		fontWeight("600"),
		fontSize("1.3rem"),
		letterSpacing("-0.01em"),
		color("var(--text)"),
	)
	// The progress "loader": full-width, a touch taller, with a serif percent.
	rule(".bento-goals .goal-card-loader",
		height("46px"),
		margin("0.5rem 0 0"),
		borderRadius("12px"),
	)
	rule(".bento-goals .goal-card-loader .budget-amount",
		fontSize("0.95rem"),
	)
	rule(".bento-goals .goal-card-loader .budget-pct",
		fontFamily(serif),
		fontSize("1.05rem"),
		fontWeight("600"),
	)

	// Figures grid: the key numbers as scannable stat cells (redesign replaces the
	// run-on "$X to go · by date · save $X/mo" sentence). Auto-fit columns spread across
	// the full-width card, then wrap on narrow content columns.
	rule(".goal-figs",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(150px, 1fr))"),
		gap("0.5rem 1.5rem"),
		margin("0.9rem 0"),
		padding("0.9rem 0"),
		borderTop(hair),
		borderBottom(hair),
	)
	rule(".goal-fig",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
	)
	rule(".goal-fig-k",
		fontSize("0.66rem"),
		letterSpacing("0.09em"),
		prop("text-transform", "uppercase"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".goal-fig-v",
		fontFamily(serif),
		fontSize("1.3rem"),
		fontWeight("600"),
		lineHeight("1.15"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	// Quiet meta strip (linked account, earmark coverage, interest ETA, over-fund note).
	// Collapses entirely when a goal has nothing secondary to say.
	rule(".goal-meta",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		marginBottom("0.15rem"),
	)
	rule(".goal-meta:empty",
		display("none"),
	)

	// --- Task 2: OPTIONAL contribution planner (opt-in disclosure) ---------------
	// The planner is hidden by default; this chip reveals the slider inline. Aligned
	// left, quiet by default, accented when expanded.
	rule(".goal-plan-toggle",
		alignSelf("flex-start"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		marginTop("0.5rem"),
		padding("0.35rem 0.8rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		fontWeight("500"),
		cursor("pointer"),
		transition("border-color 0.14s ease, color 0.14s ease, background 0.14s ease"),
	)
	rule(".goal-plan-toggle:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		color("var(--text)"),
		background("color-mix(in srgb, var(--bg-elev) 70%, transparent)"),
	)
	rule(".goal-plan-toggle[aria-expanded=\"true\"]",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
	)
	// When revealed, the planner sits just under its toggle with a little air.
	rule(".bento-goals .goal-plan",
		marginTop("0.6rem"),
	)

	// --- Task 4: EARMARKS made visual --------------------------------------------
	// Account exposure as cards with an earmarked-vs-free coverage bar, so the split is
	// glanceable rather than a plain number table.
	rule(".ea-exp-list",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".ea-acct",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		padding("0.8rem 0.95rem"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
	)
	rule(".ea-acct-top",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.4rem 1rem"),
	)
	rule(".ea-acct-name",
		fontWeight("600"),
		color("var(--text)"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".ea-acct-figs",
		display("inline-flex"),
		alignItems("baseline"),
		gap("0.9rem"),
		flexWrap("wrap"),
	)
	rule(".ea-acct-earmarked",
		fontFamily(serif),
		fontSize("1.05rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		color("var(--accent)"),
	)
	rule(".ea-acct .ea-exp-free",
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
		fontSize("0.85rem"),
	)
	// The coverage bar: an earmarked segment over a free track.
	rule(".ea-acct-bar",
		position("relative"),
		height("10px"),
		borderRadius("999px"),
		overflow("hidden"),
		background(track),
	)
	rule(".ea-bar-fill",
		position("absolute"),
		top("0"),
		left("0"),
		height("100%"),
		borderRadius("999px"),
		background("var(--accent)"),
		transition("width 0.25s ease"),
	)
	rule(".ea-bar-fill.is-over",
		background("var(--danger)"),
	)
	// Per-goal coverage bar in each goal's earmark block header, beside the % chip.
	rule(".ea-cover",
		// Same bar treatment as .ea-acct-bar (10px, same track/fill) — one coverage-bar
		// language across both earmark views, just inline-width here vs full-width there.
		position("relative"),
		width("140px"),
		height("10px"),
		borderRadius("999px"),
		overflow("hidden"),
		background(track),
		flexShrink("0"),
	)
	rule(".ea-cover-fill",
		position("absolute"),
		top("0"),
		left("0"),
		height("100%"),
		borderRadius("999px"),
		background("var(--accent)"),
		transition("width 0.25s ease"),
	)

	// "Earmarks by goal": each goal's account rows are NESTED under its header, not flush
	// with the goal title. Indent them and hang a connector rail on the left so the
	// parent/child relationship is unambiguous (money routed to a goal — legibility matters).
	rule(".ea-goals",
		display("flex"),
		flexDirection("column"),
		gap("1.1rem"),
	)
	rule(".ea-goal-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem 0.75rem"),
		marginBottom("0.5rem"),
	)
	rule(".ea-goal-name",
		flex("1 1 auto"),
		minWidth("0"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".ea-goal-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		marginLeft("0.4rem"),
		paddingLeft("1rem"),
		borderLeft("2px solid color-mix(in srgb, var(--border) 80%, transparent)"),
	)
	rule(".ea-row",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
	)
	rule(".ea-row-acct",
		flex("1 1 auto"),
		minWidth("0"),
		color("var(--text-dim)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".ea-row-amt",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		color("var(--text)"),
	)
}
