// SPDX-License-Identifier: MIT

package styles

// registerAnnualGridPlanSurface emits the W6 forward-planning layer on the annual
// budget grid (C371/C393/C394): a distinct wash for FUTURE months, a "projected"
// figure style for cells pre-filled from recurring bills + goal plans, the income
// SCENARIO bar and its underfunded-cell highlight, and the plan/actual/projected
// legend. Theme tokens only; layered AFTER registerAnnualGridSurface so these
// win over the base cell rules where they overlap.
func registerAnnualGridPlanSurface() {
	// --- future-month wash (C371) — past months are actuals, future months read
	// as a lighter "yet to happen" band so the eye lands on where planning starts.
	rule(".budget-annualgrid-th.is-future",
		color("var(--text-dim)"),
		background("color-mix(in srgb, var(--text-dim) 6%, var(--bg-elev))"),
	)
	rule(".budget-annualgrid-td.is-future",
		background("color-mix(in srgb, var(--text-dim) 4%, transparent)"),
	)
	// The current-month band must still win over the future wash on its own column.
	rule(".budget-annualgrid-td.is-current.is-future",
		background("color-mix(in srgb, var(--accent) 6%, transparent)"),
	)

	// --- projected figure (C394) — a cell pre-filled from schedules, visually
	// distinct from a real Actual (solid) and from the quiet Plan line: a muted,
	// dotted-underlined number that reads as "expected, not yet spent".
	rule(".budget-annualgrid-projected",
		fontWeight("600"),
		color("var(--accent)"),
		lineHeight("1.15"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("border-bottom", "1px dotted color-mix(in srgb, var(--accent) 55%, transparent)"),
	)

	// --- scenario bar (C393) — a compact ephemeral control row above the matrix.
	rule(".budget-annualgrid-scenario",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		padding("0.4rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		fontSize("var(--type-13)"),
	)
	rule(".budget-annualgrid-scenario.is-on",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".budget-annualgrid-scenario-label",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".budget-annualgrid-scenario-delta",
		minWidth("4.5rem"),
		textAlign("center"),
		fontWeight("700"),
		prop("font-variant-numeric", "tabular-nums"),
		color("var(--accent)"),
	)
	// The scenario step buttons + reset reuse the app .btn chrome; this just tightens
	// them into the bar.
	rule(".budget-annualgrid-scenario .btn",
		padding("0.15rem 0.5rem"),
	)
	rule(".budget-annualgrid-scenario-status",
		marginLeft("auto"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		fontWeight("600"),
	)
	rule(".budget-annualgrid-scenario-status.is-under",
		color("var(--warn, #e6a23c)"),
	)
	rule(".budget-annualgrid-scenario-status.is-clear",
		color("var(--text-dim)"),
	)

	// --- underfunded cell highlight (C393) — an amber wash + marker, deliberately
	// NOT the red overspend tone, so "won't be funded in this scenario" reads as a
	// caution rather than a committed overspend. Layered after is-over so it wins in
	// a scenario view.
	rule(".budget-annualgrid-td.is-underfunded",
		background("color-mix(in srgb, var(--warn, #e6a23c) 18%, transparent)"),
		prop("box-shadow", "inset 2px 0 0 var(--warn, #e6a23c)"),
	)
	rule(".budget-annualgrid-td.is-underfunded .budget-annualgrid-projected, .budget-annualgrid-td.is-underfunded .budget-annualgrid-actual",
		color("var(--warn, #e6a23c)"),
	)

	// --- legend (C394) — a small key so the three cell states are self-explaining.
	rule(".budget-annualgrid-legend",
		display("flex"),
		alignItems("center"),
		gap("0.9rem"),
		flexWrap("wrap"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
	)
	rule(".budget-annualgrid-legend-item",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		whiteSpace("nowrap"),
	)
	rule(".budget-annualgrid-swatch",
		width("0.7rem"),
		height("0.7rem"),
		borderRadius("3px"),
		flexShrink("0"),
	)
	rule(".budget-annualgrid-swatch.is-actual",
		background("var(--text)"),
	)
	rule(".budget-annualgrid-swatch.is-planned",
		background("var(--text-faint)"),
	)
	rule(".budget-annualgrid-swatch.is-projected",
		background("color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("border", "1px dotted var(--accent)"),
	)
	rule(".budget-annualgrid-swatch.is-under",
		background("color-mix(in srgb, var(--warn, #e6a23c) 40%, transparent)"),
	)

	// Overflow-fit pass (2026-07-19 review, lane E #3): compact cells + a
	// top-anchored scroll cue for the wide 12-month matrix. Chained here so it
	// layers after the base grid rules it refines.
	registerAnnualGridFit()
}
