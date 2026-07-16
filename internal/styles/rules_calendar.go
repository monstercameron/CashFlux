// SPDX-License-Identifier: MIT

package styles

// registerCalendarSurface emits the chrome for the reusable calendar primitive
// (internal/ui/calendar.go): a header nav row (prev / month-title / next), a
// weekday-label row, and a 7-column month grid of day cells. All selectors use a
// dedicated `.uical-` prefix so this shared primitive never collides with the
// existing transactions calendar (`.cal-cell`/`.txn-cal-cell`). Registered by the
// styles coordinator (do not touch install.go). Theme-token colours only
// (var(--text)/(--border)/(--bg-card)/(--bg-elev)/--accent); never
// var(--fg)/(--line)/(--dim)/(--faint) (undefined -> render dark).
func registerCalendarSurface() {
	// Shared tokens: a faint hairline and a calm hover wash derived from the theme.
	const (
		hair  = "1px solid var(--border)"
		wash  = "color-mix(in srgb, var(--bg-elev) 70%, transparent)"
		faint = "color-mix(in srgb, var(--text) 55%, transparent)"
	)

	// Root: a self-contained calendar block. A CSS var carries the cell min-height so
	// the compact variant can shrink every cell by overriding one value.
	rule(".uical",
		prop("--uical-cell-h", "78px"),
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		width("100%"),
		color("var(--text)"),
	)
	rule(".uical.is-compact",
		prop("--uical-cell-h", "40px"),
	)

	// Header: prev button, centered month/year title, next button.
	rule(".uical-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".uical-nav-btn",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("2rem"),
		height("2rem"),
		padding("0"),
		borderRadius("9px"),
		border(hair),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color 0.15s ease, background 0.15s ease, color 0.15s ease"),
	)
	rule(".uical-nav-btn:hover:not(:disabled)",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background(wash),
	)
	rule(".uical-nav-btn:disabled",
		opacity("0.4"),
		cursor("default"),
	)
	rule(".uical-nav-btn svg",
		width("1.05rem"),
		height("1.05rem"),
	)
	rule(".uical-title",
		flex("1 1 auto"),
		textAlign("center"),
		fontWeight("600"),
		fontSize("1.02rem"),
		letterSpacing("-0.01em"),
		color("var(--text)"),
		whiteSpace("nowrap"),
	)

	// Weekday label row + the day grid share one 7-column template so headers line up
	// exactly over their columns.
	rule(".uical-weekdays",
		display("grid"),
		gridTemplateColumns("repeat(7, 1fr)"),
		gap("0.25rem"),
	)
	rule(".uical-weekday",
		prop("text-transform", "uppercase"),
		textAlign("center"),
		fontSize("0.66rem"),
		letterSpacing("0.08em"),
		fontWeight("600"),
		color(faint),
		padding("0.15rem 0"),
	)
	rule(".uical-grid",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
	)
	rule(".uical-week",
		display("grid"),
		gridTemplateColumns("repeat(7, 1fr)"),
		gap("0.25rem"),
	)

	// Day cell: a bordered card whose day number sits top-left and optional content
	// (badges/markers) stacks beneath. Buttons and static divs share the look.
	rule(".uical-cell",
		display("flex"),
		flexDirection("column"),
		alignItems("stretch"),
		gap("0.2rem"),
		minHeight("var(--uical-cell-h)"),
		padding("0.35rem 0.4rem"),
		border(hair),
		borderRadius("10px"),
		background("var(--bg-card)"),
		color("var(--text)"),
		textAlign("left"),
		font("inherit"),
		cursor("default"),
		transition("border-color 0.15s ease, background 0.15s ease, box-shadow 0.15s ease"),
	)
	// Clickable cells (rendered as <button>) get a hover affordance + a keyboard ring.
	rule("button.uical-cell",
		cursor("pointer"),
	)
	rule("button.uical-cell:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background(wash),
	)
	rule("button.uical-cell:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)

	// Out-of-month padding days: dimmed and recessed so the anchored month reads first.
	rule(".uical-cell.is-out",
		background("transparent"),
		borderColor("color-mix(in srgb, var(--border) 55%, transparent)"),
		color(faint),
	)
	rule(".uical-cell.is-out .uical-daynum",
		color(faint),
		fontWeight("400"),
	)

	// Today: an accent ring (inset box-shadow so it doesn't shift layout).
	rule(".uical-cell.is-today",
		borderColor("var(--accent)"),
		boxShadow("inset 0 0 0 1px var(--accent)"),
	)
	rule(".uical-cell.is-today .uical-daynum",
		color("var(--accent)"),
	)

	// Selected: a filled accent wash + stronger border, distinct from the today ring
	// (both can apply at once — selected wins the fill, today keeps its ring).
	rule(".uical-cell.is-selected",
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 16%, var(--bg-card))"),
	)
	rule("button.uical-cell.is-selected:hover",
		background("color-mix(in srgb, var(--accent) 24%, var(--bg-card))"),
	)

	// The day number chip and the optional content well beneath it.
	rule(".uical-daynum",
		fontSize("0.82rem"),
		fontWeight("600"),
		lineHeight("1.1"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".uical-daycontent",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("0"),
		marginTop("auto"),
		fontSize("0.7rem"),
		color(faint),
	)
	rule(".uical-daycontent:empty",
		display("none"),
	)

	// Compact variant tightens padding + type so a date-picker footprint stays snug.
	rule(".uical.is-compact .uical-cell",
		padding("0.2rem 0.25rem"),
		borderRadius("8px"),
	)
	rule(".uical.is-compact .uical-daynum",
		fontSize("0.78rem"),
	)
	rule(".uical.is-compact .uical-daycontent",
		fontSize("0.62rem"),
	)
}
