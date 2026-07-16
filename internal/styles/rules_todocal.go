// SPDX-License-Identifier: MIT

package styles

// registerTodoCalSurface emits the To-do surface's view-switcher (list / board /
// calendar segmented control) and the calendar schedule chrome (per-day task chips
// rendered inside the reusable .uical grid). Registered after registerGenerated so
// same-selector rules here win. Theme tokens only (var(--text)/(--border)/(--bg-card)/
// (--bg-elev)/--accent); never var(--fg)/(--line)/(--dim)/(--faint).
func registerTodoCalSurface() {
	// --- View switcher (segmented control) ---------------------------------------
	rule(".todo-viewswitch",
		display("inline-flex"),
		alignItems("center"),
		gap("2px"),
		padding("3px"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-elev) 50%, transparent)"),
	)
	rule(".tvw-btn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		padding("0.3rem 0.7rem"),
		border("1px solid transparent"),
		borderRadius("8px"),
		background("transparent"),
		color("color-mix(in srgb, var(--text) 72%, transparent)"),
		fontSize("0.82rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background 0.15s ease, color 0.15s ease, border-color 0.15s ease"),
	)
	rule(".tvw-btn:hover",
		color("var(--text)"),
		background("color-mix(in srgb, var(--text) 6%, transparent)"),
	)
	// Selected segment: a quiet tinted/outlined state — NOT a solid accent fill, which
	// is reserved for the one primary CTA per screen (Add task). This keeps the switch
	// from competing with the primary action for the eye.
	rule(".tvw-btn.is-active",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)

	// --- Calendar schedule view --------------------------------------------------
	rule(".tcal",
		display("block"),
		width("100%"),
	)
	// Day-cell task chips stack under the day number; the reusable .uical-cell owns the
	// number + click target, we only style the content column.
	rule(".tcal-daytasks",
		display("flex"),
		flexDirection("column"),
		gap("2px"),
		marginTop("2px"),
		width("100%"),
	)
	rule(".tcal-chip",
		display("flex"),
		alignItems("center"),
		gap("0.3rem"),
		width("100%"),
		padding("1px 5px"),
		border("1px solid transparent"),
		borderRadius("5px"),
		background("color-mix(in srgb, var(--bg-elev) 70%, transparent)"),
		color("var(--text)"),
		fontSize("0.72rem"),
		lineHeight("1.35"),
		textAlign("left"),
		cursor("pointer"),
		overflow("hidden"),
		transition("background 0.12s ease, border-color 0.12s ease"),
	)
	rule(".tcal-chip:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
	)
	rule(".tcal-chip-dot",
		flexShrink("0"),
		width("6px"),
		height("6px"),
		borderRadius("50%"),
		background("color-mix(in srgb, var(--text) 32%, transparent)"),
	)
	rule(".tcal-chip.p-high .tcal-chip-dot", background("var(--danger, #d8716f)"))
	rule(".tcal-chip.p-med .tcal-chip-dot", background("var(--accent)"))
	rule(".tcal-chip.p-low .tcal-chip-dot", background("color-mix(in srgb, var(--text) 28%, transparent)"))
	rule(".tcal-chip-title",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".tcal-chip.is-done .tcal-chip-title",
		textDecoration("line-through"),
		color("color-mix(in srgb, var(--text) 55%, transparent)"),
	)
	rule(".tcal-more",
		display("block"),
		padding("0 5px"),
		fontSize("0.68rem"),
		fontWeight("600"),
		color("color-mix(in srgb, var(--text) 60%, transparent)"),
	)
	// Day cell content column (chips stacked over the add affordance).
	rule(".tcal-daycell",
		display("flex"),
		flexDirection("column"),
		gap("2px"),
		width("100%"),
	)
	// "Schedule a task here" — a quiet "+" revealed on cell hover / keyboard focus, so an
	// empty calendar stays clean but a task is one click away on any day.
	rule(".tcal-add",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		alignSelf("flex-start"),
		width("22px"),
		height("22px"),
		padding("0"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		background("color-mix(in srgb, var(--bg-card) 80%, transparent)"),
		color("color-mix(in srgb, var(--text) 70%, transparent)"),
		cursor("pointer"),
		// A faint but PRESENT default so empty days read as actionable at a glance
		// (not fully hidden until hover); it firms up on hover / keyboard focus.
		opacity("0.4"),
		transition("opacity 0.12s ease, background 0.12s ease, border-color 0.12s ease, color 0.12s ease"),
	)
	rule(".uical-cell:hover .tcal-add, .tcal-add:focus-visible, .tcal-add:hover",
		opacity("1"),
	)
	rule(".tcal-add:hover",
		borderColor("var(--accent)"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-card))"),
	)
}
