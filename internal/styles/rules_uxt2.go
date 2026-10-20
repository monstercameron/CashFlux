// SPDX-License-Identifier: MIT

package styles

// registerUXT2 gives Transactions a compact four-step type scale and a single,
// quiet ledger surface. All selectors are limited to the transaction widgets or
// classes that are unique to the Transactions page.
func registerUXT2() {
	// The controls sit directly on the page canvas. The ledger is the one framed
	// surface, with a fine edge and no elevation.
	rule(`.bento > .w[data-widget="txn-toolbar"]`,
		background("transparent"),
		border("0"),
		borderRadius("0"),
		boxShadow("none"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] > .wbody`,
		padding("0"),
	)
	rule(`.bento > .w[data-widget="txn-table"]`,
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		boxShadow("none"),
	)

	// Four deliberate sizes and two weights. Fraunces remains the display face
	// for the page heading; the ledger itself uses the UI face for quick scanning.
	rule(`body:has(.bento-ledger) h1`,
		fontFamily("var(--font-display)"),
		fontSize("24px"),
		fontWeight("600"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-countline`,
		fontSize("14px"),
		fontWeight("400"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-countline strong`,
		fontSize("17px"),
		fontWeight("600"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-toolbar-foot, .bento > .w[data-widget="txn-toolbar"] .txn-upcoming-badge`,
		fontSize("12px"),
		fontWeight("600"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] .filter-chips-lead, .bento > .w[data-widget="txn-table"] .std-pager-size-label`,
		fontSize("12px"),
		fontWeight("400"),
		color("var(--text-dim)"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-table`,
		fontSize("14px"),
		fontWeight("400"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-table thead th`,
		fontSize("12px"),
		fontWeight("600"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .row-desc-text, .bento > .w[data-widget="txn-table"] .txn-description-edit`,
		fontSize("14px"),
		fontWeight("400"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .td-amount`,
		fontSize("14px"),
		fontWeight("600"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-review-pill, .bento > .w[data-widget="txn-table"] .txn-scheduled-pill, .bento > .w[data-widget="txn-table"] .txn-upcoming-badge, .bento > .w[data-widget="txn-table"] .txn-desc-tag`,
		fontSize("12px"),
		fontWeight("600"),
		borderRadius("999px"),
	)

	// Sentence-case labels inherit their actual copy; remove legacy uppercase
	// styling and tracking without changing the wording supplied by the UI.
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-countline, .bento > .w[data-widget="txn-toolbar"] .txn-toolbar-foot, .bento > .w[data-widget="txn-toolbar"] .txn-upcoming-badge, .bento > .w[data-widget="txn-toolbar"] .filter-chips-lead, .bento > .w[data-widget="txn-table"] .std-pager-size-label, .bento > .w[data-widget="txn-table"] .txn-table thead th, .bento > .w[data-widget="txn-table"] .txn-review-pill, .bento > .w[data-widget="txn-table"] .txn-scheduled-pill`,
		textTransform("none"),
		letterSpacing("normal"),
	)

	// The scheduled preview is an unboxed continuation of the ledger context,
	// not another card nested above the first transaction.
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-count-upcoming-rows`,
		border("0"),
		borderRadius("0"),
		boxShadow("none"),
		padding("0.25rem 0"),
	)
	rule(`.bento > .w[data-widget="txn-toolbar"] .txn-upcoming-badge`,
		border("0"),
		background("transparent"),
		padding("0"),
	)

	// Keep the selected row legible in either palette, including the red, muted,
	// and positive amounts. A narrow accent edge marks selection without a dark
	// fill; keyboard focus gets a separate light inner ring.
	rule(`.bento > .w[data-widget="txn-table"] .txn-table tbody tr.row.selected`,
		customProp("--txn-row-surface", "color-mix(in srgb, var(--accent) 8%, var(--bg-card))"),
		borderLeft("2px solid var(--accent)"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-table tbody tr.row.selected > td`,
		background("var(--txn-row-surface)"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-table tbody tr.row.selected:focus-within > td`,
		boxShadow("inset 0 0 0 2px var(--text)"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-table tbody tr.row.selected .txn-description-edit:focus-visible`,
		outline("2px solid var(--text)"),
		outlineOffset("-2px"),
	)
	rule(`.bento > .w[data-widget="txn-table"] .txn-catsuggest, .bento > .w[data-widget="txn-table"] .txn-cat-choose`,
		color("var(--up-ink)"),
	)
	rule(`.bento-ledger .txn-cal-week-head, .bento-ledger .txn-cal-week-head .cal-head, .bento-ledger .txn-cal-week-total, .bento-ledger .txn-cal-week-count, .bento-ledger .txn-cal-legend, .bento-ledger .txn-presets-label`,
		fontSize("12px"),
		fontWeight("400"),
		color("var(--text-dim)"),
		textTransform("none"),
		letterSpacing("normal"),
	)

	// Ledger controls use a compact corner. The toolbar above the money summary
	// has its own control geometry in registerTxnControls.
	rule(`.bento > .w[data-widget="txn-table"] button`,
		borderRadius("6px"),
	)
}
