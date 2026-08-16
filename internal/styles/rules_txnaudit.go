// SPDX-License-Identifier: MIT

package styles

// registerTxnAudit emits the Transactions-page audit expansion's new surfaces
// (C560–C572): the ledger period/view bar, the row's Edit affordance, the grouped
// row menu's section headings, the light-theme correction for destructive menu
// items, and the quick-filter scope note. Theme tokens only — no hardcoded
// colours outside the one legacy hover this file is correcting.
func registerTxnAudit() {
	// --- C572: the row menu groups by what an action costs ---------------------
	// A section heading doubles as the separator: the hairline above it is the rule
	// between tiers, so the menu needs one device rather than two. The first heading
	// in a menu drops its rule (nothing above it to separate from).
	rule(".add-menu-section",
		marginTop(".3rem"),
		paddingTop(".35rem"),
		paddingRight(".7rem"),
		paddingBottom(".15rem"),
		paddingLeft(".7rem"),
		borderTop("1px solid var(--border)"),
		color("var(--text-faint)"),
		fontSize("var(--type-11)"),
		fontWeight("600"),
		letterSpacing(".05em"),
		textTransform("uppercase"),
		whiteSpace("nowrap"),
	)
	rule(".add-menu-section:first-child",
		marginTop("0"),
		paddingTop("0"),
		borderTop("0"),
	)
	// The existing `.add-item.danger` hover is a dark-theme maroon that reads as a
	// black smear on a light background. Give the light theme its own tint from the
	// negative token rather than leaving the destructive item's hover unreadable.
	rule("[data-theme=\"light\"] .add-item.danger:hover, [data-theme=\"light\"] .add-item.danger:focus-visible",
		background("color-mix(in srgb, var(--cf-neg,#c0392b) 12%, transparent)"),
		color("var(--cf-neg,#c0392b)"),
	)

	// --- C563: the row's Edit affordance ---------------------------------------
	// A pencil beside the ⋯ trigger. It is icon-only ON PURPOSE: the ledger's actions
	// column is deliberately narrow so the description keeps the reading width, and a
	// visible "Edit" word either clipped the ⋯ out of the cell or took that width from
	// the description. The pencil is the universal edit affordance, carries its name
	// for assistive tech and a tooltip for the mouse, and is in the tab order — which
	// is the whole gap: the row's click target was invisible AND unreachable by
	// keyboard. The column grows just enough for two icon buttons.
	rule(".txn-table .td-actions",
		whiteSpace("nowrap"),
	)
	// The ⋯ trigger is a plain `.btn` and carried full button padding (~44px) — with
	// the pencil beside it that overran the fixed actions column by a few pixels and
	// clipped the menu trigger's right edge. Match the icon-button padding the cell
	// already applies to its other glyph buttons, so both fit the column as it stands
	// instead of the column growing into the description.
	rule(".txn-table .td-actions .add-wrap > .btn",
		padding("0.2rem 0.35rem"),
		minHeight("0"),
	)
	rule(".txn-row-edit",
		display("inline-flex"),
		alignItems("center"),
		marginRight(".1rem"),
		color("var(--text-dim)"),
	)
	rule(".txn-row-edit:hover, .txn-row-edit:focus-visible",
		color("var(--accent)"),
	)
	// The word stays in the accessibility tree and out of the layout.
	rule(".txn-row-edit-label",
		position("absolute"),
		width("1px"),
		height("1px"),
		padding("0"),
		margin("-1px"),
		overflow("hidden"),
		whiteSpace("nowrap"),
		border("0"),
	)

	// --- C568: the quick-filter counts say what they count ---------------------
	// A full-width note under the chips (flex-basis:100% breaks it onto its own line
	// inside the wrapping preset row) rather than a tooltip, so the qualification is
	// present at rest for everyone.
	rule(".txn-presets-note",
		flexBasis("100%"),
		marginTop(".15rem"),
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)

	// --- C560: the ledger's own period + view bar ------------------------------
	// One row: the month stepper on the left, the Ledger/Calendar pair on the right,
	// wrapping to two lines on a narrow pane. It reads as part of the ledger's chrome
	// (a quiet hairline beneath), never as a second toolbar competing with the first.
	rule(".txn-scopebar",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap(".5rem"),
		paddingTop(".1rem"),
		paddingBottom(".45rem"),
	)
	rule(".txn-scopebar-spacer",
		flex("1 1 auto"),
	)
	// The month label is the bar's one piece of typographic weight: it is the answer
	// to "what am I looking at", so it is set larger and tabular so stepping months
	// does not shift the chevrons beside it.
	rule(".txn-scope-label",
		minWidth("9.5rem"),
		textAlign("center"),
		fontWeight("600"),
		fontSize("var(--type-14)"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
		whiteSpace("nowrap"),
	)
	rule(".txn-scope-step",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.75rem"),
		height("1.75rem"),
		padding("0"),
		borderRadius("var(--radius-md)"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".txn-scope-step:hover, .txn-scope-step:focus-visible",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".txn-scope-step[disabled]",
		opacity(".45"),
		cursor("default"),
	)
	// Ledger / Calendar: a segmented pair, so which view is showing is legible from
	// the fill rather than from which one is missing. aria-pressed carries the same
	// fact to assistive tech.
	rule(".txn-viewtoggle",
		display("inline-flex"),
		alignItems("center"),
		borderRadius("var(--radius-pill)"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		overflow("hidden"),
	)
	rule(".txn-viewtoggle-btn",
		display("inline-flex"),
		alignItems("center"),
		gap(".3rem"),
		padding(".25rem .7rem"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		font("inherit"),
		fontSize("var(--type-13)"),
		cursor("pointer"),
		whiteSpace("nowrap"),
	)
	rule(".txn-viewtoggle-btn:hover",
		color("var(--text)"),
	)
	rule(".txn-viewtoggle-btn[aria-pressed=\"true\"]",
		background("var(--accent)"),
		color("#fff"),
	)
	ruleContentMax(contentGrid1, ".txn-viewtoggle-btn span",
		position("absolute"),
		width("1px"),
		height("1px"),
		padding("0"),
		margin("-1px"),
		overflow("hidden"),
		whiteSpace("nowrap"),
		border("0"),
	)

	// --- C569: the ledger says it is re-sorting until the new order is painted --
	// The rows dim while a sort is in flight, so a fast click-and-read cannot mistake
	// the old order for the new one. Kept subtle: this is a sub-second state.
	rule(".data-table[aria-busy=\"true\"] tbody",
		opacity(".55"),
		transition("opacity .12s ease"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".data-table[aria-busy=\"true\"] tbody",
		transition("none"),
	)
}
