// SPDX-License-Identifier: MIT

package styles

// registerTodoPolish styles the 2026-07-19 To-do workspace polish pass: a single,
// zoned command bar (replacing the old two-row toolbar), the quick-view segmented
// control's count badges, and the relocated, collapsible "Suggested for you"
// section that now sits below the user's committed tasks. Registered after
// registerGenerated() so it wins the cascade; all tones come from theme tokens so
// light and dark both read correctly.
func registerTodoPolish() {
	// One command bar of two deterministic rows (never a single wrapping line whose
	// overflow scatters the controls): the TOP row is identity + primary action —
	// search grows to fill the slack, the Add-task/More cluster is pinned right; the
	// BOTTOM row is view + filters — the display switch and quick-view lens sit left,
	// the sort/filter pills group right. Only the filter cluster wraps at narrow widths.
	rule(".todo-cmdbar",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
		marginBottom("0.6rem"),
	)
	rule(".todo-cmdbar .cmdbar-row",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		minWidth("0"),
	)
	// Search owns the top row's slack; its pill lifts the base max-width cap.
	rule(".todo-cmdbar .cmdbar-top .todo-ctrl-search",
		flex("1 1 auto"),
		maxWidth("none"),
		minWidth("8rem"),
	)
	// The primary action cluster holds its natural size at the far edge.
	rule(".todo-cmdbar .cmdbar-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginLeft("auto"),
		flex("none"),
	)
	// The views row may wrap as a whole at narrow widths; the filter pills right-align
	// and wrap as one cluster first, keeping the two switches anchored on the left.
	rule(".todo-cmdbar .cmdbar-views",
		flexWrap("wrap"),
	)
	rule(".todo-cmdbar .cmdbar-filters",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		justifyContent("flex-end"),
		gap("0.5rem"),
		marginLeft("auto"),
	)

	// Quick-view + display-view segmented controls share the .todo-viewswitch chrome; a
	// count badge rides each quick-view chip (e.g. "Overdue 3").
	rule(".tvw-btn .tvw-count",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		minWidth("1.1rem"),
		marginLeft("0.3rem"),
		padding("0 0.3rem"),
		borderRadius("var(--radius-pill)"),
		fontSize("0.65rem"),
		fontWeight("700"),
		lineHeight("1.4"),
		background("color-mix(in srgb, var(--text) 12%, transparent)"),
		color("var(--text)"),
	)
	// The overdue badge picks up the app's negative tone so a backlog reads at a glance
	// (colour PLUS the word "Overdue", never colour alone).
	rule(".tvw-btn.is-overdue .tvw-count",
		background("color-mix(in srgb, var(--money-negative) 20%, transparent)"),
		color("var(--money-negative)"),
	)

	// "Suggested for you" — relocated BELOW the committed list, in a bordered,
	// collapsible section so the user's own tasks always lead.
	rule(".todo-suggest",
		marginTop("0.75rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		overflow("hidden"),
	)
	rule(".todo-suggest-head",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		width("100%"),
		padding("0.55rem 0.75rem"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		fontWeight("600"),
		fontSize("var(--type-13)"),
		cursor("pointer"),
		textAlign("left"),
	)
	rule(".todo-suggest-head:hover",
		background("color-mix(in srgb, var(--text) 5%, transparent)"),
	)
	rule(".todo-suggest-caret",
		marginLeft("auto"),
		color("var(--text-dim)"),
		transition("transform 0.15s ease"),
	)
	rule(".todo-suggest-head[aria-expanded=\"true\"] .todo-suggest-caret",
		transform("rotate(90deg)"),
	)
	rule(".todo-suggest-body",
		display("grid"),
		gap("0.35rem"),
		padding("0 0.75rem 0.6rem"),
	)
}
