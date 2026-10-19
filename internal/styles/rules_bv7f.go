// SPDX-License-Identifier: MIT

package styles

// registerBV7F keeps the delayed budget-save notice on the shared secondary
// type scale, with steady-width money figures.
func registerBV7F() {
	rule(".toast-msg",
		fontSize("var(--fs-meta)"),
		fontWeight("400"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
	)
}
