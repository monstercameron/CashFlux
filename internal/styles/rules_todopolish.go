// SPDX-License-Identifier: MIT

package styles

// registerTodoPolish styles the 2026-07-19 To-do workspace polish pass: a single,
// zoned command bar (replacing the old two-row toolbar), the quick-view segmented
// control's count badges, and the relocated, collapsible "Suggested for you"
// section that now sits below the user's committed tasks. Registered after
// registerGenerated() so it wins the cascade; all tones come from theme tokens so
// light and dark both read correctly.
func registerTodoPolish() {
	// One command bar: a single row that wraps, so the surface reads as a focused
	// workspace rather than a scattered tool collection. Zones — left (search + active
	// view), middle (quick views + sort + filters), right (primary Add + More).
	rule(".todo-cmdbar",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
		marginBottom("0.6rem"),
	)
	rule(".todo-cmdbar .cmdbar-group",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
	)
	// The left zone grows so the search box fills the slack; its search pill lifts the
	// base max-width cap and fills the remaining room.
	rule(".todo-cmdbar .cmdbar-left",
		flex("1 1 16rem"),
		minWidth("0"),
	)
	rule(".todo-cmdbar .cmdbar-left .todo-ctrl-search",
		flex("1 1 auto"),
		maxWidth("none"),
		minWidth("8rem"),
	)
	// The right zone is pushed to the far edge and holds its natural size — the single
	// primary action plus the More menu, isolated from the filters.
	rule(".todo-cmdbar .cmdbar-right",
		marginLeft("auto"),
		flex("none"),
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
		borderRadius("999px"),
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
		borderRadius("8px"),
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
		fontSize("0.8rem"),
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
