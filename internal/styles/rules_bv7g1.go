// SPDX-License-Identifier: MIT

package styles

// registerBV7G1 keeps the Edit budget effective-cap note readable beneath Limit.
func registerBV7G1() {
	rule(".budget-edit-effective-cap",
		fontSize("var(--fs-meta, 12.5px)"),
		lineHeight("1.4"),
		margin("0.25rem 0 0"),
	)
}
