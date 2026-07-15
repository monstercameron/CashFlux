// SPDX-License-Identifier: MIT

package styles

// registerBudgetTargets emits the styles for per-budget funding targets (BG1) and
// the quick-fill chip strip (BG4) in the budget edit form. The chips read as quiet,
// tappable pills that name a figure and show its value, so picking one is informed.
// Kept in its own file (registered after the generated sheet) so it never touches
// the shared budgets surface rules.
func registerBudgetTargets() {
	// The funding-target editor block sits calmly under the amount/period row.
	rule(".budget-target-section",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		marginTop("0.35rem"),
	)

	// Quick-fill: a labelled row above the chip strip.
	rule(".budget-fill-row",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		marginTop("0.2rem"),
	)
	rule(".budget-fill-chips",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	// Each chip: a bordered pill, accent-tinted on hover, showing "Label · $value".
	rule(".budget-fill-chip",
		display("inline-flex"),
		alignItems("baseline"),
		gap("0.15rem"),
		padding("0.3rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		fontSize("0.8rem"),
	)
	rule(".budget-fill-chip:hover",
		borderColor("var(--accent)"),
		background("var(--hover)"),
	)
	rule(".budget-fill-chip-label",
		fontWeight("600"),
	)
}
