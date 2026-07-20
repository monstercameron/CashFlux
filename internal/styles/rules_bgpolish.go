// SPDX-License-Identifier: MIT

package styles

// registerBgPolish styles the 2026-07-19 Budgets/Goals first-viewport polish: the
// Budgets "Needs attention" top-problems strip and the Goals payday-funding review
// banner. Theme tokens only (--text / --text-dim / --border / --bg-card / --danger),
// so both light and dark track automatically.
func registerBgPolish() {
	// The strip is a tight card of at most three rows — the real work, above the fold.
	rule(".bgattn",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".bgattn-head",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginBottom("0.15rem"),
	)
	rule(".bgattn-title",
		fontWeight("650"),
		color("var(--text)"),
	)
	rule(".bgattn-sub",
		fontSize("var(--type-14)"),
		color("var(--text-dim)"),
	)
	// One problem row: name + figures on the left, a status pill and one action right.
	rule(".bgattn-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.5rem 0.6rem"),
		borderRadius("0.5rem"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		// A tone stripe down the left edge keyed to severity.
		boxShadow("inset 3px 0 0 var(--border)"),
	)
	rule(".bgattn-row.is-over",
		boxShadow("inset 3px 0 0 var(--danger)"),
	)
	rule(".bgattn-row.is-near",
		boxShadow("inset 3px 0 0 rgba(245,158,11,0.9)"),
	)
	rule(".bgattn-row.is-pace",
		boxShadow("inset 3px 0 0 rgba(245,158,11,0.55)"),
	)
	rule(".bgattn-main",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".bgattn-cat",
		fontWeight("600"),
		color("var(--text)"),
		overflow("hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".bgattn-nums",
		fontSize("var(--type-14)"),
		color("var(--text-dim)"),
	)
	rule(".bgattn-over",
		color("var(--danger)"),
		fontWeight("600"),
	)
	// Status pill — small, uppercase, tone-keyed.
	rule(".bgattn-pill",
		flexShrink("0"),
		fontSize("var(--type-11)"),
		fontWeight("700"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		padding("0.15rem 0.45rem"),
		borderRadius("var(--radius-pill)"),
	)
	rule(".bgattn-pill.is-over",
		background("rgba(216,113,111,0.18)"),
		color("var(--danger)"),
	)
	rule(".bgattn-pill.is-near",
		background("rgba(245,158,11,0.20)"),
		color("#d98c00"),
	)
	rule(".bgattn-pill.is-pace",
		background("rgba(245,158,11,0.14)"),
		color("#d98c00"),
	)
	rule("[data-theme=\"light\"] .bgattn-pill.is-near",
		color("#b45309"),
	)
	rule("[data-theme=\"light\"] .bgattn-pill.is-pace",
		color("#b45309"),
	)

	// --- Goals: payday-funding review banner (collapsed by default) ---
	rule(".wf-review-banner",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		width("100%"),
		padding("0.55rem 0.7rem"),
		borderRadius("0.6rem"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		font("inherit"),
		prop("text-align", "left"),
	)
	rule(".wf-review-banner .wf-review-icon",
		fontSize("1.05rem"),
		flexShrink("0"),
	)
	rule(".wf-review-title",
		fontWeight("650"),
	)
	rule(".wf-review-ready",
		marginLeft("0.15rem"),
		color("var(--text-dim)"),
		fontSize("0.9rem"),
	)
	rule(".wf-review-chev",
		marginLeft("auto"),
		flexShrink("0"),
		color("var(--text-dim)"),
	)
}
