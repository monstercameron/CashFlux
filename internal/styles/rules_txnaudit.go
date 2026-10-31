// SPDX-License-Identifier: MIT

package styles

// registerTxnAudit emits the Transactions-page audit expansion's new surfaces
// (C560–C572): the ledger period/view bar, the row's Edit affordance, the grouped
// row menu's section headings, the light-theme correction for destructive menu
// items, and the quick-filter scope note. Theme tokens only — no hardcoded
// colours outside the one legacy hover this file is correcting.
func registerTxnAudit() {
	// The description takes the spare width and truncates. The status marks sit in one fixed,
	// right-aligned column so the same marks line up down the ledger.
	rule(".bento-ledger .txn-table .txn-desc-mainline > .row-desc-text",
		flex("1 1 auto"),
	)
	rule(".bento-ledger .txn-table .txn-desc-status",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-end"),
		gap("0.35rem"),
		flex("0 0 auto"),
		minWidth("7.5rem"),
		whiteSpace("nowrap"),
	)
	// Every ledger row is one line high. Secondary detail (follow-ups, the note, the automatic
	// category line) opens from the row's info or note popover, so nothing grows a row. The
	// description clips and its tags truncate on the main line.
	rule(".bento-ledger .txn-table tbody tr.row",
		height("3rem"),
	)
	rule(".bento-ledger .txn-table tbody tr.row > td",
		verticalAlign("middle"),
	)
	rule(".bento-ledger .txn-table tbody tr.row > td.row-desc-cell",
		overflow("hidden"),
	)
	rule(".bento-ledger .txn-table .txn-desc-mainline",
		flexWrap("nowrap"),
		alignItems("center"),
		minWidth("0"),
		overflow("hidden"),
	)
	rule(".bento-ledger .txn-table .txn-desc-mainline > *",
		flexShrink("0"),
	)
	rule(".bento-ledger .txn-table .txn-desc-mainline > .row-desc-text, .bento-ledger .txn-table .txn-desc-mainline > .txn-desc-account",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		flexShrink("1"),
	)
	rule(".bento-ledger .txn-table .txn-desc-tags",
		display("inline-flex"),
		flexWrap("nowrap"),
		minWidth("0"),
		overflow("hidden"),
	)
	rule(".txn-row-info-line, .txn-row-note-pop",
		fontSize("var(--fs-meta)"),
		color("var(--text-dim)"),
		lineHeight("1.45"),
		whiteSpace("pre-wrap"),
		maxWidth("22rem"),
	)
	rule(".txn-row-note-pop",
		padding("0.4rem 0.6rem"),
	)
	// The row's Edit and ⋯ buttons sit in one group with the Accounts row gap between them.
	rule(".txn-row-actions",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-end"),
		gap("0.75rem"),
	)
	// Each action keeps its own size: the group spaces them and never shrinks one.
	rule(".txn-row-actions > *",
		flex("0 0 auto"),
	)
	// The ledger's title sits on the left of its count line, as the Accounts list title does.
	rule(".txn-table-head",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.25rem 1rem"),
		marginBottom("0.5rem"),
	)
	rule(".txn-table-head .card-title",
		margin("0"),
	)
	// The cleared mark is a quiet inline glyph beside the description. It is not a control.
	rule(".txn-row-glyph",
		display("inline-flex"),
		alignItems("center"),
		flex("0 0 auto"),
		color("var(--text-dim)"),
	)
	// The revealed note sits on its own line under the description, in the quiet meta size.
	rule(".txn-row-note",
		flex("0 0 100%"),
		marginTop("0.35rem"),
		padding("0.4rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		fontSize("var(--fs-meta)"),
		whiteSpace("pre-wrap"),
	)
	// The period control is the shared toolbar buttons around one select. Its custom-range
	// editor is positioned against the control, so the control is the anchor.
	rule(".txn-scopebar",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".txn-period-control",
		display("flex"),
		alignItems("center"),
		position("relative"),
		gap("0.4rem"),
	)
	// The period select takes the toolbar button's tokens, so it sits at the same height and
	// weight as the buttons beside it. Scoped to beat the generic select rule.
	rule(".txn-scopebar select.txn-period-select.field",
		minHeight("var(--tool-h)"),
		padding("var(--btn-py) var(--btn-px)"),
		borderRadius("var(--radius-lg)"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		fontSize("var(--fs-body)"),
		fontWeight("500"),
	)
	// C1-15: keep the transaction row that owns an open action menu visually
	// connected to the popover, including when the menu flips above its trigger.
	rule(".txn-table tbody tr:has(.overflow-menu-open)",
		background("color-mix(in srgb, var(--accent) 9%, var(--bg-card))"),
	)
	rule(".txn-table tbody tr:has(.overflow-menu-open) > td",
		background("transparent"),
	)

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
		fontSize("var(--fs-meta)"),
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
	// --- The header select-all checkbox ----------------------------------------
	// It sits in the header cell above the per-row boxes, so it aligns with them
	// rather than centring in the cell like a column label would.
	rule(".txn-table thead .txn-selectall-box",
		display("block"),
		margin("0"),
		cursor("pointer"),
	)

	// --- C568: the quick-filter counts say what they count ---------------------
	// A full-width note under the chips (flex-basis:100% breaks it onto its own line
	// inside the wrapping preset row) rather than a tooltip, so the qualification is
	// present at rest for everyone.
	rule(".txn-presets-note",
		flexBasis("100%"),
		marginTop(".15rem"),
		color("var(--text-faint)"),
		fontSize("var(--fs-meta)"),
	)

	// --- C560: the ledger's own period + view bar ------------------------------
	// One row: the month stepper on the left, the Ledger/Calendar pair on the right,
	// The month label is the bar's one piece of typographic weight: it is the answer
	// to "what am I looking at", so it is set larger and tabular so stepping months
	// does not shift the chevrons beside it.
	rule(".txn-scope-label",
		minWidth("9.5rem"),
		textAlign("center"),
		fontWeight("600"),
		fontSize("var(--fs-body)"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
		whiteSpace("nowrap"),
	)
	// Ledger / Calendar: a segmented pair, so which view is showing is legible from
	// the fill rather than from which one is missing. aria-pressed carries the same

	// --- C569: the ledger says it is re-sorting until the new order is painted --
	// The rows dim while a sort is in flight, so a fast click-and-read cannot mistake
	// the old order for the new one. Kept subtle: this is a sub-second state.
	// C628: the same treatment, driven by a class the filter toolbar owns outright.
	// The search-pending flag is application state, and raising it costs a full
	// re-render of a screen heavy enough to stall for 300ms — longer than the flag
	// itself lives, so the render that would have made the rows inert usually never
	// happened. The toolbar sets this class in the keystroke instead, and clears it
	// on the render where the query has committed. Kept separate from aria-busy so
	// clearing it can never cancel a sort that is genuinely still in flight.
	rule(".data-table.is-search-stale tbody, .data-table[aria-busy=\"true\"] tbody",
		opacity(".55"),
		transition("opacity .12s ease"),
		// C625: dimming said "these rows are stale" and then let you click one
		// anyway. The window is sub-second, which is exactly long enough for the
		// fast click the report describes — reading the old order, clicking the row
		// you saw, and landing on whichever row the new order put there. Inert for
		// the duration is the only version of this that cannot be raced. The header
		// keeps its pointer events, so a second sort click is rejected by the
		// table's own re-entrancy guard rather than by being unreachable.
		pointerEvents("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".data-table.is-search-stale tbody, .data-table[aria-busy=\"true\"] tbody",
		transition("none"),
	)

	// A row that belongs to a split group carries an accent bar on its leading edge.
	rule(".row.txn-ledger-row.txn-row-grouped",
		boxShadow("inset 3px 0 0 0 var(--accent)"),
	)
	// The "Scheduled" pill on a ghost row reads as plain 12px text.
	rule(".txn-scheduled-pill.txn-scheduled-pill-plain",
		fontSize("12px"),
		textTransform("none"),
	)
	// One entry in the transaction history panel.
	rule(".rows > .row.txn-history-entry",
		display("block"),
		padding("0.5rem 0"),
	)
}
