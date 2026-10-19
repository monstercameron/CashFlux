// SPDX-License-Identifier: MIT

package styles

// registerRulesBV7E keeps budget target and commitment details readable in both
// row layouts while following the page's shared type scale.
func registerRulesBV7E() {
	rule(".bento-budgets [data-testid^=budget-target-]",
		fontSize("var(--fs-meta)"), fontWeight("400"), lineHeight("1.4"),
	)
	rule(".bento-budgets [data-testid^=budget-free-after-committed-]",
		fontSize("var(--fs-meta)"), fontWeight("400"), lineHeight("1.4"),
	)
}
