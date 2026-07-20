// SPDX-License-Identifier: MIT

package styles

// registerAnnualGridSurface emits the BG9 annual budget grid: a categories × 12-months
// plan-vs-actual matrix. The design goal is scannability — hairline row dividers in a
// bordered scroll frame, a clear actual-over-plan hierarchy per cell, an accent-banded
// current-month column, and a faint red wash on overspent cells (a heatmap-lite read).
// Sticky header row, first column, and footer keep the labels and totals in view while
// scrolling. Theme tokens only.
func registerAnnualGridSurface() {
	rule(".budget-annualgrid",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
	)
	// Header: the section toggle on the left, the year stepper on the right.
	rule(".budget-annualgrid-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		flexWrap("wrap"),
	)
	// The toggle reads as a full-width collapsible-section header, not a stray button:
	// a rotating disclosure caret, the title, and a right-aligned hint of what it opens.
	rule(".budget-annualgrid-toggle",
		prop("appearance", "none"),
		fontFamily("inherit"),
		cursor("pointer"),
		flex("1 1 auto"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.55rem"),
		padding("0.5rem 0.8rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		color("var(--text)"),
		textAlign("left"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".budget-annualgrid-toggle:hover",
		borderColor("var(--text-dim)"),
		background("color-mix(in srgb, var(--bg-elev) 65%, transparent)"),
	)
	rule(".budget-annualgrid-toggle:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// The disclosure caret points right when closed, rotates down when open.
	rule(".budget-annualgrid-caret",
		display("inline-flex"),
		flexShrink("0"),
		color("var(--text-dim)"),
		transition("transform 0.15s ease"),
	)
	rule(".budget-annualgrid-caret.is-open",
		transform("rotate(90deg)"),
	)
	rule(".budget-annualgrid-toggle-label",
		fontWeight("600"),
	)
	rule(".budget-annualgrid-toggle-hint",
		marginLeft("auto"),
		color("var(--text-dim)"),
		fontSize("var(--type-13)"),
		whiteSpace("nowrap"),
	)
	rule(".budget-annualgrid-year",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
	)
	rule(".budget-annualgrid-yearlabel",
		minWidth("3rem"),
		textAlign("center"),
		fontWeight("700"),
		fontSize("0.95rem"),
		prop("font-variant-numeric", "tabular-nums"),
		color("var(--text)"),
	)

	// Bordered scroll frame so the matrix reads as one contained object.
	rule(".budget-annualgrid-scroll",
		overflowX("auto"),
		maxWidth("100%"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		scrollbarWidth("thin"),
	)
	rule(".budget-annualgrid-table",
		// separate (not collapse) so sticky cells keep their borders and backgrounds.
		prop("border-collapse", "separate"),
		prop("border-spacing", "0"),
		width("max-content"),
		minWidth("100%"),
		fontSize("var(--type-13)"),
		prop("font-variant-numeric", "tabular-nums"),
	)

	// --- header row ---------------------------------------------------------------
	rule(".budget-annualgrid-table thead th",
		position("sticky"),
		top("0"),
		zIndex("3"),
		background("var(--bg-elev)"),
		padding("0.5rem 0.7rem"),
		textAlign("right"),
		whiteSpace("nowrap"),
		fontSize("var(--type-11)"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		borderBottom("1px solid var(--border)"),
	)
	// The top-left corner: sticky on BOTH axes, above everything, left-aligned.
	rule(".budget-annualgrid-corner",
		position("sticky"),
		left("0"),
		zIndex("5"),
		background("var(--bg-elev)"),
		textAlign("left"),
		borderRight("1px solid var(--border)"),
	)

	// --- body rows ----------------------------------------------------------------
	// Sticky first column: the budget name, on the card surface so it reads as a label
	// rail distinct from the data.
	rule(".budget-annualgrid-rowhead",
		position("sticky"),
		left("0"),
		zIndex("2"),
		background("var(--bg-card)"),
		padding("0.4rem 0.75rem"),
		textAlign("left"),
		whiteSpace("nowrap"),
		fontWeight("600"),
		color("var(--text)"),
		borderRight("1px solid var(--border)"),
		borderBottom("1px solid var(--border-subtle)"),
	)
	rule(".budget-annualgrid-td",
		padding("0.3rem 0.7rem"),
		textAlign("right"),
		whiteSpace("nowrap"),
		borderBottom("1px solid var(--border-subtle)"),
	)
	// Row hover aids horizontal scanning across a wide year.
	rule(".budget-annualgrid-tr:hover .budget-annualgrid-td",
		background("var(--hover)"),
	)
	rule(".budget-annualgrid-tr:hover .budget-annualgrid-rowhead",
		background("color-mix(in srgb, var(--hover) 60%, var(--bg-card))"),
	)

	// --- current-month accent band ------------------------------------------------
	rule(".budget-annualgrid-th.is-current",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 14%, var(--bg-elev))"),
	)
	rule(".budget-annualgrid-td.is-current",
		background("color-mix(in srgb, var(--accent) 6%, transparent)"),
	)
	rule(".budget-annualgrid-tr:hover .budget-annualgrid-td.is-current",
		background("color-mix(in srgb, var(--accent) 12%, var(--hover))"),
	)

	// --- overspent cells (heatmap-lite) — defined AFTER is-current so it wins ------
	rule(".budget-annualgrid-td.is-over",
		background("color-mix(in srgb, var(--down, #d8716f) 10%, transparent)"),
	)
	rule(".budget-annualgrid-td.is-over .budget-annualgrid-actual",
		color("var(--down, #d8716f)"),
	)

	// --- the Total column + row (divider + emphasis) ------------------------------
	rule(".budget-annualgrid-th.is-total, .budget-annualgrid-td.is-total",
		borderLeft("1px solid var(--border)"),
		fontWeight("700"),
		color("var(--text)"),
	)
	// Footer: sticky totals row.
	rule(".budget-annualgrid-table tfoot td, .budget-annualgrid-table tfoot th",
		position("sticky"),
		bottom("0"),
		zIndex("3"),
		background("var(--bg-elev)"),
		padding("0.5rem 0.7rem"),
		textAlign("right"),
		whiteSpace("nowrap"),
		fontWeight("700"),
		color("var(--text)"),
		borderTop("1px solid var(--border)"),
	)
	rule(".budget-annualgrid-corner.is-foot",
		zIndex("5"),
		textAlign("left"),
	)

	// --- the plan-vs-actual cell --------------------------------------------------
	rule(".budget-annualgrid-cell",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-end"),
		gap("0"),
		width("100%"),
		padding("0.1rem 0.15rem"),
		border("0"),
		background("transparent"),
		color("inherit"),
		font("inherit"),
		cursor("pointer"),
		borderRadius("var(--radius-sm)"),
	)
	rule(".budget-annualgrid-cell:hover",
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".budget-annualgrid-cell:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "-2px"),
	)
	// Actual is the primary figure; plan rides beneath it, smaller and quiet.
	rule(".budget-annualgrid-actual",
		fontWeight("600"),
		color("var(--text)"),
		lineHeight("1.15"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".budget-annualgrid-plan",
		fontSize("0.72em"),
		color("var(--text-faint)"),
		lineHeight("1.1"),
		prop("font-variant-numeric", "tabular-nums"),
	)

	// The W6 forward-planning layer (future wash, projected figures, income
	// scenario bar, underfunded highlight, legend) is emitted right after the base
	// grid so its overlapping rules win. Kept in its own file (rules_annualgridplan.go).
	registerAnnualGridPlanSurface()
}
