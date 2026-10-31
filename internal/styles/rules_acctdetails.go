// SPDX-License-Identifier: MIT

package styles

// registerAcctDetails lays out the per-account "Details" panel (accounts_row.go detailsNode).
// The panel is a two-column grid that fills the row's width: the period figures run across the
// top, the balance chart and the 90-day projection sit on the left, the reading and the account's
// facts sit on the right, and the documents and the institution share a footer line. Below 760px
// it becomes a single column. Registered from Register().
func registerAcctDetails() {
	// The row's head starts with a type icon and a gap, so the name sits one icon plus one gap in.
	// Everything under the head (the recurring line, the details panel) indents to that same
	// column, so the nested content reads as belonging to the name rather than to the row edge.
	rule(".acct-row",
		customProp("--acct-head-icon", "1rem"),
		customProp("--acct-head-gap", "0.75rem"),
		customProp("--acct-indent", "calc(var(--acct-head-icon) + var(--acct-head-gap))"),
	)
	rule(".acct-row > :not(.acct-row-head)",
		marginLeft("var(--acct-indent)"),
		minWidth("0"),
	)
	rule(".acct-row-details",
		display("grid"),
		gridTemplateColumns("minmax(0, 1.6fr) minmax(0, 1fr)"),
		gap("1.25rem 2.5rem"),
		alignItems("start"),
		marginTop("0.75rem"),
		paddingTop("1rem"),
		borderTop("1px solid var(--border-subtle)"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 760px)", ".acct-row-details",
		gridTemplateColumns("minmax(0, 1fr)"),
	)
	rule(".acct-row-details > *",
		minWidth("0"),
		maxWidth("100%"),
	)

	// The period figures: three equal columns across the top, each a quiet label over the amount.
	rule(".acct-flow",
		gridColumn("1 / -1"),
		display("grid"),
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		gap("0.75rem 2rem"),
		fontVariantNumeric("tabular-nums"),
	)
	ruleMedia("(max-width: 760px)", ".acct-flow",
		gridTemplateColumns("repeat(auto-fit, minmax(9rem, 1fr))"),
		gap("0.75rem 1rem"),
	)
	rule(".acct-fig",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
	)
	rule(".acct-fig-label",
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
	)
	rule(".acct-flow-value",
		fontSize("var(--type-18)"),
		fontWeight("500"),
		lineHeight("1.25"),
	)

	// The left and right columns stack their items with one even rhythm.
	rule(".acct-panel-chart, .acct-panel-side",
		display("flex"),
		flexDirection("column"),
		gap("0.9rem"),
		minWidth("0"),
	)
	// The projection sits under the history chart as a continuation, so it drops its own divider.
	rule(".acct-panel-chart .acct-fwd",
		borderTop("0"),
		marginTop("0"),
		paddingTop("0"),
	)

	// The footer: documents and the institution nudge on one line. An open documents drawer
	// takes the full line, and the institution action sits at the far edge.
	rule(".acct-panel-footer",
		gridColumn("1 / -1"),
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.75rem 1.5rem"),
		minWidth("0"),
	)
	rule(".acct-panel-footer > .acct-docs:has(> .acct-docs-body)",
		flexBasis("100%"),
	)
	rule(".acct-panel-footer .acct-set-institution",
		marginLeft("auto"),
	)

	// Tool buttons in the panel keep their natural width instead of stretching to the column.
	rule(".acct-row-details .btn-tool",
		width("auto"),
		alignSelf("flex-start"),
	)
	// The 90-day balance sparkline and its figure sit on one row, centred vertically.
	rule(".acct-spark-fig",
		display("inline-flex"),
		alignItems("center"),
		gap("0.6rem"),
	)
	rule(".acct-spark-fig .acct-spark",
		flexShrink("0"),
	)
	// The projection sparkline sits inline beside its reading, so it drops the chart's top
	// margin and keeps its width.
	rule(".acct-fwd-spark",
		marginTop("0"),
		flexShrink("0"),
	)
	// The "Low point" line with its toggle: one wrapping line. The drivers list opens under it
	// on a line of its own.
	rule(".acct-proj",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.35rem 0.75rem"),
		minWidth("0"),
	)
	rule(".acct-proj > .acct-proj-drivers",
		flexBasis("100%"),
	)
	// The link from an investment account to its holdings: one line, at the row's secondary size.
	rule(".acct-holdings-link",
		fontSize("var(--type-14)"),
		whiteSpace("nowrap"),
	)
	// The institution tag on a row: a dot and a name that truncate instead of widening the row.
	rule(".inst-chip",
		minWidth("0"),
		maxWidth("100%"),
		fontSize("var(--type-13)"),
	)
	// The editor's collapsed "Details" disclosure keeps a gap above and below.
	rule(".acct-edit-fold",
		margin("0.6rem 0"),
	)

	// The services list (subscriptions on the Accounts page): the monthly total above the rows,
	// each row a name and provider, its monthly cost and a remove button.
	rule(".acct-services",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		minWidth("0"),
	)
	rule(".acct-services-total",
		margin("0"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".acct-service-row",
		alignItems("center"),
		gap("0.75rem"),
	)
	rule(".acct-service-cost",
		marginLeft("auto"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)

	// The institution swatch beside a name: a round dot drawn in the institution's colour,
	// which the element supplies as --inst-color (the colour is user data).
	rule(".inst-dot",
		display("inline-block"),
		width("8px"),
		height("8px"),
		borderRadius("50%"),
		background("var(--inst-color)"),
		flexShrink("0"),
	)
	rule(".inst-dot.inst-dot-lg",
		width("10px"),
		height("10px"),
	)
	// A row's left edge takes its institution's colour, supplied as --acct-inst-color. Rows
	// without an institution keep the plain row.
	rule(".row.acct-row-inst",
		borderLeft("3px solid var(--acct-inst-color)"),
	)

	// Hint and caption lines in the account forms keep a fixed gap to what they sit against.
	rule(".t-caption.acct-gap-below, .budget-sub.acct-gap-below",
		margin("0 0 0.5rem"),
	)
	rule(".t-caption.acct-hint-roomy",
		margin("0 0 0.75rem"),
	)
	rule(".t-caption.acct-hint-tuck",
		margin("-0.35rem 0 0"),
	)
	rule(".t-caption.acct-hint-label",
		margin("0.5rem 0 0"),
	)
	rule(".t-caption.acct-docs-empty",
		margin("0.35rem 0"),
	)
	rule(".t-caption.acct-docs-hint",
		margin("0.25rem 0 0"),
	)
	// A block that follows another in the reconcile panel drops a little below it.
	rule(".acct-block-above",
		margin("0.75rem 0 0 0"),
	)

	// The transfer form's currency note and balance preview.
	rule(".budget-sub.acct-fx-note",
		margin("0"),
	)
	rule(".budget-sub.acct-xfer-preview",
		display("grid"),
		gap("0.15rem"),
		margin("0"),
	)
	// The liquidity line: buckets in a wrapping row.
	rule(".t-caption.acct-liquidity",
		display("flex"),
		flexWrap("wrap"),
		gap("0.25rem 1rem"),
	)

	// The ownership split editor: a member name beside a percentage field and its suffix.
	rule(".form-grid.acct-share-row",
		gridTemplateColumns("1fr auto"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".labeled-field.acct-share-field",
		display("flex"),
		flexDirection("row"),
		alignItems("center"),
		gap("0.25rem"),
	)
	rule(".labeled-field .field.acct-share-input",
		width("5rem"),
	)

	// The editor's four-figure balance strip: current, cleared, statement and projected, with
	// the provenance note on a line of its own.
	rule(".acct-bal-strip",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem 1.25rem"),
		padding("0.5rem 0.65rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		margin("0 0 0.6rem"),
	)
	rule(".acct-bal-stat",
		display("grid"),
		gap("0.1rem"),
	)
	rule(".acct-bal-prov",
		flex("1 1 100%"),
	)

	// The reconcile panel: the resolve actions, the history heading and its rows.
	rule(".acct-recon-actions",
		display("flex"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".acct-recon-hist-head",
		display("flex"),
		justifyContent("space-between"),
		alignItems("baseline"),
		gap("1rem"),
	)
	rule(".row.acct-recon-hist-row",
		display("flex"),
		justifyContent("space-between"),
		gap("1rem"),
	)

	// The stacked label-over-control field used by the account forms: a tighter gap than the
	// base .labeled-field. The group variant is a fieldset with no frame, padding or margin.
	rule(".labeled-field.labeled-field-tight",
		gap("0.25rem"),
	)
	rule(".labeled-field.labeled-group",
		gap("0.25rem"),
		borderStyle("none"),
		margin("0"),
		padding("0"),
		minWidth("0"),
	)
	rule(".labeled-group > legend",
		padding("0"),
	)

	// The document label field grows to share its line with the expiry date and the attach button.
	rule(".field.acct-doc-label",
		flex("1 1 12rem"),
	)
}
