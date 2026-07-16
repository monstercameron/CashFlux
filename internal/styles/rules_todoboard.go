// SPDX-License-Identifier: MIT

package styles

// registerTodoBoardSurface emits the chrome for the to-do BOARD (kanban) view:
// a horizontally-scrolling row of column cards, each with a header (title +
// count) and a stack of task cards. Registered by the styles installer AFTER the
// generated defaults, so same-selector rules here win. All selectors are prefixed
// `.tdb-` so the board's styles never collide with the list/calendar views. Colours
// use ONLY the theme tokens (var(--text)/(--border)/(--bg-card)/(--bg-elev)/--accent)
// so the board reads correctly in both light and dark themes.
func registerTodoBoardSurface() {
	const (
		hair  = "1px solid color-mix(in srgb, var(--border) 60%, transparent)"
		quiet = "color-mix(in srgb, var(--text) 60%, transparent)"
		faint = "color-mix(in srgb, var(--text) 40%, transparent)"
	)

	// --- The board: a horizontal, scrollable flex row of columns ------------------
	rule(".tdb-wrap",
		display("flex"),
		flexDirection("row"),
		alignItems("flex-start"),
		gap("1rem"),
		overflowX("auto"),
		padding("0.25rem 0.1rem 1rem"),
	)

	// --- A column card ------------------------------------------------------------
	rule(".tdb-col",
		display("flex"),
		flexDirection("column"),
		flexShrink("0"),
		minWidth("260px"),
		maxWidth("320px"),
		gap("0.65rem"),
		padding("0.85rem 0.8rem 1rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	// Column header: the title on the left, a count pill trailing.
	rule(".tdb-col-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
		paddingBottom("0.5rem"),
		borderBottom(hair),
	)
	rule(".tdb-col-title",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("600"),
		fontSize("0.9rem"),
		letterSpacing("0.01em"),
		color("var(--text)"),
	)
	rule(".tdb-count",
		flexShrink("0"),
		minWidth("1.4rem"),
		prop("text-align", "center"),
		padding("0.1rem 0.5rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--text) 10%, transparent)"),
		color(quiet),
		fontSize("0.75rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)

	// The task-card stack within a column.
	rule(".tdb-col-body",
		display("flex"),
		flexDirection("column"),
		gap("0.55rem"),
	)
	rule(".tdb-empty",
		padding("0.9rem 0.4rem"),
		color(faint),
		fontSize("0.82rem"),
		prop("text-align", "center"),
	)

	// --- A task card --------------------------------------------------------------
	rule(".tdb-card",
		position("relative"),
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		padding("0.7rem 0.75rem"),
		border("1px solid var(--border)"),
		borderRadius("11px"),
		background("var(--bg-card)"),
		cursor("pointer"),
		transition("border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease"),
	)
	rule(".tdb-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 35%, var(--border))"),
		background("color-mix(in srgb, var(--bg-card) 88%, var(--accent))"),
		boxShadow("0 1px 3px color-mix(in srgb, var(--text) 12%, transparent)"),
		transform("translateY(-1px)"),
	)
	rule(".tdb-card:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	// Top row of a card: the priority dot leads the title.
	rule(".tdb-card-top",
		display("flex"),
		alignItems("flex-start"),
		gap("0.5rem"),
	)
	rule(".tdb-card-title",
		flex("1 1 auto"),
		minWidth("0"),
		color("var(--text)"),
		fontSize("0.88rem"),
		fontWeight("500"),
		lineHeight("1.3"),
	)
	// A struck-through, dimmed title once the task is done.
	rule(".tdb-card.is-done .tdb-card-title",
		color(quiet),
		prop("text-decoration", "line-through"),
	)

	// The priority dot — a small round chip that encodes priority by HUE (not an
	// opacity ramp), matching the calendar chips exactly so "medium" reads the same
	// on both surfaces: high = danger, medium = accent, low = faint text-mix.
	rule(".tdb-prio-dot",
		flexShrink("0"),
		width("9px"),
		height("9px"),
		marginTop("0.28rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--text) 28%, transparent)"),
	)
	rule(".tdb-prio-dot[data-prio=\"high\"]",
		background("var(--danger, #d8716f)"),
	)
	rule(".tdb-prio-dot[data-prio=\"med\"]",
		background("var(--accent)"),
	)
	rule(".tdb-prio-dot[data-prio=\"low\"]",
		background("color-mix(in srgb, var(--text) 28%, transparent)"),
	)

	// Card footer: the due chip on the left, the "Next" advance affordance trailing.
	rule(".tdb-card-foot",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".tdb-due",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		padding("0.12rem 0.5rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--text) 8%, transparent)"),
		color(quiet),
		fontSize("0.72rem"),
		fontWeight("500"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	// An overdue due chip reads as urgent via the shared danger token — never the
	// accent (which is the app's "selected/branded" colour, not a warning).
	rule(".tdb-due.is-overdue",
		background("color-mix(in srgb, var(--danger, #d8716f) 16%, transparent)"),
		color("var(--danger, #d8716f)"),
		fontWeight("600"),
	)

	// The one-click "advance to next column" affordance.
	rule(".tdb-next",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		marginLeft("auto"),
		padding("0.18rem 0.55rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color(quiet),
		fontSize("0.72rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("border-color 0.14s ease, color 0.14s ease, background 0.14s ease"),
	)
	rule(".tdb-next:hover",
		borderColor("color-mix(in srgb, var(--accent) 50%, var(--border))"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
	)
}
