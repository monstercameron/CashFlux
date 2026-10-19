// SPDX-License-Identifier: MIT

package styles

// registerBR2Card keeps full-card figures, progress, and pace guidance in normal flow.
func registerBR2Card() {
	rule(".bento-budgets .budget-card-loader-figs",
		position("static"),
		height("auto"),
		minHeight("0"),
		margin("0.35rem 0 0"),
		padding("0"),
		background("transparent"),
		prop("text-shadow", "none"),
	)
	rule(".bento-budgets .budget-card-loader",
		position("relative"),
		height("5px"),
		minHeight("5px"),
		margin("0.35rem 0 0"),
		overflow("hidden"),
	)
	rule(".bento-budgets .budget-card-loader .bar-fill",
		height("100%"),
	)
	rule(".bento-budgets .budget-pace-legend",
		display("block"),
		position("relative"),
		margin("0.35rem 0 0"),
	)
	rule(".bento-budgets .budget-todos-head",
		width("fit-content"),
		minWidth("0"),
	)
}
