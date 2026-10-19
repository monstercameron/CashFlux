// SPDX-License-Identifier: MIT

package styles

// registerRulesBV6B keeps the mobile budgets toolbar in normal flow and gives
// the primary action its own full-width row.
func registerRulesBV6B() {
	ruleMedia("(max-width: 600px)", ".budlist-head .budgets-tb",
		width("100%"),
		maxWidth("100%"),
		margin("0"),
		minHeight("0"),
	)
	ruleMedia("(max-width: 600px)", ".budlist-head .budgets-tb .filter-toolbar-actions",
		display("grid"),
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		alignItems("stretch"),
		gap("0.5rem"),
		width("100%"),
		minWidth("0"),
		height("auto"),
		minHeight("0"),
		margin("0"),
		padding("0"),
		position("static"),
	)
	ruleMedia("(max-width: 600px)", ".budlist-head .budgets-tb .filter-toolbar-actions > #budgets-settings-wrap, .budlist-head .budgets-tb .filter-toolbar-actions > #budgets-tools-wrap, .budlist-head .budgets-tb .filter-toolbar-actions > [data-testid=budgets-density]",
		minWidth("0"),
		width("100%"),
		margin("0"),
	)
	ruleMedia("(max-width: 600px)", ".budlist-head .budgets-tb .filter-toolbar-actions > [data-testid=budgets-add]",
		gridColumn("1 / -1"),
		width("100%"),
		minHeight("44px"),
		margin("0"),
		justifyContent("center"),
	)
}
