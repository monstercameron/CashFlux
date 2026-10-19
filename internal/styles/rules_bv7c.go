// SPDX-License-Identifier: MIT

package styles

// registerBV7C gives the headline's date label the shared quiet meta scale.
func registerBV7C() {
	rule(".bento-budgets .bud3-headline-window",
		fontSize("var(--fs-meta)"), fontWeight("400"), lineHeight("1.35"), color("var(--text-dim)"),
	)
}
