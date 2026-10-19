// SPDX-License-Identifier: MIT

package styles

// registerRulesBR2TB keeps the Budgets toolbar readable and gives small
// disclosure controls comfortable touch targets.
func registerRulesBR2TB() {
	rule(".bud2-tools-menu",
		minWidth("260px"),
	)
	rule(".bud2-tools-menu .add-item",
		whiteSpace("nowrap"),
	)
	rule(".bud2-member-share",
		fontSize("var(--type-14)"),
		lineHeight("1.45"),
		flex("1 1 100%"),
	)
	rule(".budget-agemoney-why",
		minWidth("24px"),
		minHeight("24px"),
	)
	rule(".budget-issues-chevbtn",
		minWidth("24px"),
		minHeight("24px"),
	)
	rule(".budget-issues-rail.is-followups-only",
		background("var(--bg-card)"),
		borderColor("var(--border)"),
		color("var(--muted)"),
	)
	ruleMedia("(pointer: coarse)", ".budget-issues-chevbtn",
		minWidth("44px"),
		minHeight("44px"),
	)
	ruleMedia("(pointer: coarse)", ".budget-agemoney-why",
		minWidth("44px"),
		minHeight("44px"),
	)
}
