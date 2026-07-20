// SPDX-License-Identifier: MIT

package styles

// registerAnnualGridFit reduces — and, where it can't be eliminated, clearly
// signals from the TOP — the horizontal overflow of the budget "Plan the year"
// annual grid (2026-07-19 v1.2.7 review, lane E #3).
//
// The categories x 12-months matrix is ~1348px wide; in a ~965px expanded-sidebar
// pane the rightmost months are cut off and the only affordance is a scrollbar at
// the very bottom of a tall grid — easy to miss, so a user reads the grid as
// "eight months" and never learns the rest exist. (The category column and the
// month header are already sticky — verified in rules_annualgrid.go: the corner /
// rowhead stick left and thead th sticks top — so the labels stay put; the gap is
// purely the missing top signal + the sheer width.)
//
// Two moves, layered after the base grid (chained from registerAnnualGridPlanSurface):
//
//  1. COMPACT CELLS — tighten every cell's horizontal padding and drop the table
//     type one step, so twelve months pack into far less width (helps every pane,
//     and lets wider ones fit the whole year outright).
//  2. TOP SCROLL CUE — a quiet, top-anchored "scroll for the full year ->" line
//     rendered above the scroll frame (budgets_annualgrid.go), shown only while
//     the pane is narrow enough that the year likely still overflows and hidden
//     once it comfortably fits, so the signal is honest rather than permanent.
//
// Theme tokens only.
func registerAnnualGridFit() {
	// --- 1. compact cells ---------------------------------------------------------
	// Denser padding + one type step down; the matrix is a scan grid, so tighter
	// cells read fine and buy back a lot of width (~0.6rem/side across ~13 columns).
	rule(".budget-annualgrid-table",
		fontSize("var(--type-12)"),
	)
	rule(".budget-annualgrid-table thead th",
		padding("0.4rem 0.4rem"),
	)
	rule(".budget-annualgrid-td",
		padding("0.25rem 0.4rem"),
	)
	rule(".budget-annualgrid-rowhead",
		padding("0.35rem 0.5rem"),
	)
	rule(".budget-annualgrid-table tfoot td, .budget-annualgrid-table tfoot th",
		padding("0.4rem 0.4rem"),
	)

	// --- 2. top scroll cue --------------------------------------------------------
	// A small, right-aligned hint sitting directly above the scroll frame — the
	// top-anchored signal the bottom-only scrollbar lacked. aria-hidden in the
	// markup (a screen reader already reaches every cell); this is a sighted
	// "there's more to the right" nudge.
	rule(".budget-annualgrid-scrollcue",
		display("flex"),
		justifyContent("flex-end"),
		alignItems("center"),
		gap("0.3rem"),
		marginTop("-0.15rem"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
		fontWeight("600"),
		letterSpacing("0.01em"),
	)
	// Once the pane is wide enough that the compact year fits with room, the cue
	// would be misleading, so hide it there.
	ruleContentMin(1120, ".budget-annualgrid-scrollcue",
		display("none"),
	)
}
