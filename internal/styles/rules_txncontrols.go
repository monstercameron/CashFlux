// SPDX-License-Identifier: MIT

package styles

// registerTxnControls gives the Transactions search and command rows a single
// visual rhythm. It deliberately stops above the money summary.
func registerTxnControls() {
	// Each ledger tile has its own transformed stacking context. Lift the
	// toolbar tile while its date editor is open so later tiles cannot cover it.
	rule(".bento-ledger > .w:has(.txn-period-custom)",
		zIndex("var(--z-popover)"),
	)
	rule(".txn-responsive-toolbar .filter-toolbar",
		gap(".7rem"),
		marginBottom("0"),
	)
	rule(".txn-responsive-toolbar",
		display("flex"),
		flexDirection("column"),
		gap("10px"),
	)
	rule(".txn-responsive-toolbar .filter-toolbar-primary",
		gap(".7rem"),
	)
	rule(".txn-responsive-toolbar .filter-toolbar-primary .fctrl-search",
		boxSizing("border-box"),
		minHeight("var(--field-h)"),
		padding("0 .85rem"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	rule(".txn-responsive-toolbar .fctrl-search .fctrl-input",
		fontSize("var(--fs-body)"),
	)
	rule(".txn-responsive-toolbar .fctrl-search .fctrl-input::placeholder",
		color("var(--text-dim)"),
	)
	rule(".txn-responsive-toolbar .fctrl-search:focus-within",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
	)
	rule(".txn-responsive-toolbar .filter-toolbar-primary .filters-trigger",
		minHeight("44px"),
		width("auto"),
		flex("0 0 auto"),
		padding("0 1rem"),
	)
	rule(".txn-responsive-toolbar .filter-toolbar-actions",
		gap(".7rem"),
		minWidth("0"),
	)
	rule(".txn-toolbar-context",
		display("flex"),
		alignItems("center"),
		gap(".7rem"),
		minWidth("0"),
	)
	rule(".txn-toolbar-context",
		flexWrap("wrap"),
	)
	rule(".txn-responsive-toolbar .txn-period-custom",
		boxSizing("border-box"),
		width("min(22rem, calc(100vw - 2rem))"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		gap(".75rem"),
		padding("1rem"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-card)"),
		boxShadow("0 12px 36px rgba(0,0,0,.24)"),
	)
	rule(".txn-period-custom-title",
		gridColumn("1 / -1"),
		color("var(--text)"),
		fontSize("var(--fs-body)"),
		fontWeight("600"),
	)
	rule(".txn-period-date input",
		boxSizing("border-box"),
		width("100%"),
		minWidth("0"),
		height("40px"),
		padding("0 .5rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		fontSize("var(--fs-body)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".txn-period-date input:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("1px"),
	)
	rule(".txn-period-error",
		gridColumn("1 / -1"),
		color("var(--money-negative)"),
		fontSize("var(--fs-meta)"),
	)
	rule(".txn-period-custom-actions",
		gridColumn("1 / -1"),
		display("flex"),
		justifyContent("flex-end"),
		gap(".5rem"),
		paddingTop(".25rem"),
	)
	rule(".txn-period-cancel, .txn-period-apply",
		minHeight("36px"),
		padding("0 .75rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"),
		fontSize("var(--fs-body)"),
		fontWeight("600"),
		cursor("pointer"),
	)
	rule(".txn-period-cancel",
		background("transparent"),
		color("var(--text)"),
	)
	rule(".txn-period-apply",
		borderColor("var(--accent)"),
		background("var(--accent)"),
		color("var(--primary-button-ink, #102318)"),
	)
	rule(".txn-period-cancel:hover, .txn-period-apply:hover:not(:disabled)",
		prop("filter", "brightness(1.08)"),
	)
	rule(".txn-period-apply:disabled",
		opacity(".45"),
		cursor("default"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .filter-toolbar-primary",
		flexDirection("column"),
		alignItems("stretch"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .filter-toolbar-primary .fctrl-search, .txn-responsive-toolbar .filter-toolbar-primary .filters-trigger",
		width("100%"),
		flex("0 0 auto"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-toolbar-context",
		width("100%"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-period-custom",
		width("min(18rem, calc(100vw - 3rem))"),
		gridTemplateColumns("minmax(0, 1fr)"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-toolbar-add",
		gridColumn("1 / -1"),
	)
}
