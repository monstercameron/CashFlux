// SPDX-License-Identifier: MIT

package styles

// registerAccountsS1 lays out the account row and its details disclosure (accounts_row.go).
// The row is a shared EntityRow inside an .acct-entity wrapper, so the wrapper draws the
// row's divider, hover and institution edge and the inner .row sheds its own. The older
// .acct-row rules still give the wrapper its column flow and indent the lines under the
// name. Registered after the shared component rules.
func registerAccountsS1() {
	// The wrapper carries what the single .row used to: the divider above, the vertical
	// padding and the hover wash.
	rule(".acct-entity",
		padding("0.6rem 0"),
		borderTop("1px solid var(--border)"),
		transition("background 0.12s ease"),
	)
	rule(".acct-entity:first-child",
		borderTop("0"),
	)
	rule(".acct-entity:hover",
		background("var(--bg-elev)"),
	)
	rule("[data-theme=\"light\"] .acct-entity:hover",
		background("#efede8"),
	)
	// A row's left edge takes its institution's colour, supplied as --acct-inst-color.
	rule(".acct-entity.acct-row-inst",
		borderLeft("3px solid var(--acct-inst-color)"),
	)

	// The inner EntityRow keeps the wrapper's width, with no divider, padding or hover of its
	// own, and no indent: it is the line the indent is measured from.
	rule(".acct-entity.acct-row > .row",
		marginLeft("0"),
		padding("0"),
		borderTop("0"),
		flexWrap("nowrap"),
	)
	rule(".acct-entity.acct-row > .row:hover",
		background("transparent"),
	)
	rule(".acct-entity > .row .row-main",
		flex("1 1 0"),
		minWidth("0"),
	)
	// The Transactions button and the menu trigger keep their width.
	rule(".acct-entity > .row > .btn, .acct-entity > .row > .add-wrap",
		flexShrink("0"),
	)
	// Phones: the actions drop under the name and balance and line up from the left.
	ruleMedia("(max-width: 640px)", ".acct-entity.acct-row > .row",
		flexWrap("wrap"),
		rowGap("0.45rem"),
	)
	ruleMedia("(max-width: 640px)", ".acct-entity > .row .row-main",
		flex("1 1 9rem"),
	)

	// The badge line under the name: the qualifying badges, then the holdings link. A row with
	// none of them shows no line.
	rule(".acct-row-badges",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem 0.5rem"),
		minWidth("0"),
	)
	rule(".acct-row-badges:empty",
		display("none"),
	)

	// The details toggle is the chevron alone, at its natural width, not a full-width bar.
	rule(".acct-details .disclosure-btn",
		width("auto"),
		padding("0.1rem 0"),
		fontSize("var(--type-14)"),
	)

	// The period figures run across the top of the panel as three columns, each a quiet label
	// over its amount.
	rule(".acct-flow-facts",
		gridColumn("1 / -1"),
	)
	rule(".acct-flow-facts .kv",
		display("grid"),
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		gap("0.75rem 2rem"),
	)
	ruleMedia("(max-width: 760px)", ".acct-flow-facts .kv",
		gridTemplateColumns("repeat(auto-fit, minmax(9rem, 1fr))"),
		gap("0.75rem 1rem"),
	)
	rule(".acct-flow-facts .kv-row",
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.2rem"),
		padding("0"),
		borderTop("0"),
	)
	rule(".acct-flow-facts .kv-value",
		textAlign("left"),
		fontSize("var(--type-18)"),
		fontWeight("500"),
		lineHeight("1.25"),
	)

	// The "Why?" toggle sits inline after the low-point reading, and its drivers open on a line
	// of their own. The disclosure's wrapper steps aside so both are direct flex items of
	// .acct-proj.
	rule(".acct-proj-why",
		display("contents"),
	)
	rule(".acct-proj-why .disclosure-btn",
		width("auto"),
		padding("0"),
		fontSize("var(--type-14)"),
		color("var(--text-dim)"),
	)
	rule(".acct-proj-why > .disclosure-body",
		flexBasis("100%"),
	)
	rule(".acct-proj-why-hide",
		display("none"),
	)
	rule(".acct-proj-why .disclosure-btn[aria-expanded=\"true\"] .acct-proj-why-show",
		display("none"),
	)
	rule(".acct-proj-why .disclosure-btn[aria-expanded=\"true\"] .acct-proj-why-hide",
		display("inline"),
	)
}
