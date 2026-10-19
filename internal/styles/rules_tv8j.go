// SPDX-License-Identifier: MIT

package styles

// registerTV8J contains the sidebar rail typography refinements for TV8J.
func registerTV8J() {
	rule(".rail-section",
		textTransform("none"),
		letterSpacing("normal"),
		fontSize("var(--type-12)"),
		fontWeight("500"),
	)
}
