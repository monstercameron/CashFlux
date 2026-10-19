// SPDX-License-Identifier: MIT

package styles

// registerRulesBR2A11Y gives the budget links and unbudgeted action comfortable
// pointer targets, including larger targets on touch devices.
func registerRulesBR2A11Y() {
	for _, selector := range []string{
		".budget-meta-link",
		".budget-unbudgeted-all",
		".scope-banner-btn",
	} {
		rule(selector,
			minWidth("24px"),
			minHeight("24px"),
		)
		ruleMedia("(pointer: coarse)", selector,
			minWidth("44px"),
			minHeight("44px"),
		)
	}
}
