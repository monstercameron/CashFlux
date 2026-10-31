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
	rule(".txn-toolbar-context, .txn-toolbar-commands",
		display("flex"),
		alignItems("center"),
		gap(".7rem"),
		minWidth("0"),
	)
	rule(".txn-toolbar-context",
		flexWrap("wrap"),
	)
	rule(".txn-toolbar-commands",
		marginLeft("auto"),
		gap(".5rem"),
	)
	// The period stepper is one object: the select supplies the label, and the
	// adjacent arrows are its controls rather than three separate floating boxes.
	rule(".txn-responsive-toolbar .txn-scopebar",
		boxSizing("border-box"),
		height("44px"),
		gap("0"),
		flexWrap("nowrap"),
		padding("0"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-elev)"),
	)
	rule(".txn-responsive-toolbar .txn-scope-step",
		boxSizing("border-box"),
		width("40px"),
		height("42px"),
		border("0"),
		borderRadius("var(--radius-md)"),
		background("transparent"),
		color("var(--text-dim)"),
	)
	rule(".txn-responsive-toolbar .txn-scope-step:not([disabled]):hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".txn-responsive-toolbar .txn-scope-step[disabled]",
		opacity(".38"),
		cursor("default"),
	)
	rule(".txn-responsive-toolbar .txn-scope-clear",
		boxSizing("border-box"),
		width("34px"),
		height("42px"),
		border("0"),
		borderLeft("1px solid var(--border)"),
		borderRadius("0 6px 6px 0"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".txn-responsive-toolbar .txn-scope-clear:hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".txn-responsive-toolbar .txn-scope-clear:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("-2px"),
	)
	rule(".txn-responsive-toolbar .txn-period-control",
		width("10rem"),
		minWidth("10rem"),
		flexDirection("row"),
		alignItems("stretch"),
	)
	rule(".txn-responsive-toolbar .txn-scopebar .txn-period-select",
		boxSizing("border-box"),
		flex("1 1 auto"),
		width("0"),
		height("42px"),
		minHeight("42px"),
		borderTop("0"),
		borderBottom("0"),
		borderLeft("1px solid var(--border)"),
		borderRight("1px solid var(--border)"),
		borderRadius("0 !important"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		fontSize("var(--fs-body)"),
		fontWeight("600"),
	)
	rule(".txn-responsive-toolbar .txn-period-control.has-edit .txn-period-select",
		borderRight("0"),
	)
	rule(".txn-responsive-toolbar .txn-period-edit",
		boxSizing("border-box"),
		flex("0 0 36px"),
		height("42px"),
		border("0"),
		borderLeft("1px solid var(--border)"),
		borderRadius("0"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".txn-responsive-toolbar .txn-period-edit:hover, .txn-responsive-toolbar .txn-period-edit[aria-expanded=true]",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".txn-responsive-toolbar .txn-period-edit:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("-2px"),
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
	rule(".txn-responsive-toolbar .txn-scope-step:focus-visible, .txn-responsive-toolbar .txn-period-select:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("-2px"),
	)
	rule(".txn-responsive-toolbar .txn-viewtoggle",
		boxSizing("border-box"),
		minHeight("44px"),
		padding("3px"),
		gap("2px"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-elev)"),
	)
	rule(".txn-responsive-toolbar .txn-viewtoggle-btn",
		minHeight("36px"),
		gap(".4rem"),
		padding(".4rem .7rem"),
		borderRadius("var(--radius-md)"),
		fontSize("var(--fs-body)"),
		fontWeight("500"),
	)
	rule(".txn-responsive-toolbar .txn-viewtoggle-btn:not([aria-pressed=\"true\"]):hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".txn-responsive-toolbar .txn-viewtoggle-btn[aria-pressed=\"true\"]",
		background("color-mix(in srgb, var(--accent) 17%, var(--bg-card))"),
		color("var(--text)"),
		boxShadow("inset 2px 0 0 var(--accent)"),
	)
	rule(".txn-responsive-toolbar .txn-viewtoggle-btn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("-2px"),
	)
	rule(".txn-responsive-toolbar .txn-review-callout",
		boxSizing("border-box"),
		minHeight("44px"),
		gap(".55rem"),
		padding("0 .7rem"),
		fontSize("var(--fs-body)"),
		fontWeight("600"),
	)
	rule(".txn-responsive-toolbar .txn-review-callout-attention::before",
		display("none"),
	)
	rule(".txn-responsive-toolbar .txn-review-count",
		minWidth("1.6rem"),
		padding(".15rem .35rem"),
		borderRadius("var(--radius-md)"),
		background("color-mix(in srgb, var(--warn) 17%, var(--bg-elev))"),
		color("var(--warn)"),
		fontSize("var(--fs-meta)"),
		lineHeight("1.2"),
		fontVariantNumeric("tabular-nums"),
		textAlign("center"),
	)
	rule(".txn-responsive-toolbar .txn-toolbar-commands [data-testid=txn-more-btn]",
		minHeight("44px"),
		padding("0 .8rem"),
	)
	// Secondary actions share one footprint and one hover response. The primary
	// Add action keeps its accent fill; segmented controls keep their group shell.
	rule(".txn-responsive-toolbar .filters-trigger, .txn-responsive-toolbar .txn-review-callout, .txn-responsive-toolbar .txn-toolbar-commands [data-testid=txn-more-btn]",
		boxSizing("border-box"),
		minHeight("44px"),
		borderRadius("var(--radius-md)"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		transition("background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard)"),
	)
	rule(".txn-responsive-toolbar .filters-trigger:hover, .txn-responsive-toolbar .filters-trigger.is-open, .txn-responsive-toolbar .txn-review-callout:hover, .txn-responsive-toolbar .txn-toolbar-commands [data-testid=txn-more-btn]:hover",
		borderColor("color-mix(in srgb, var(--border) 72%, var(--text))"),
		background("color-mix(in srgb, var(--bg-card) 94%, var(--text))"),
	)
	ruleMedia("(max-width: 900px)", ".txn-responsive-toolbar .txn-toolbar-commands",
		marginLeft("0"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .filter-toolbar-primary",
		flexDirection("column"),
		alignItems("stretch"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .filter-toolbar-primary .fctrl-search, .txn-responsive-toolbar .filter-toolbar-primary .filters-trigger",
		width("100%"),
		flex("0 0 auto"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-toolbar-context, .txn-responsive-toolbar .txn-toolbar-commands",
		width("100%"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-toolbar-commands",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) auto"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-period-custom",
		width("min(18rem, calc(100vw - 3rem))"),
		gridTemplateColumns("minmax(0, 1fr)"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-toolbar-add",
		gridColumn("1 / -1"),
	)
	ruleMedia("(max-width: 520px)", ".txn-responsive-toolbar .txn-review-callout",
		justifyContent("space-between"),
	)
}
