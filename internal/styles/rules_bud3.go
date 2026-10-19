// SPDX-License-Identifier: MIT

package styles

// registerBud3 applies the compact-row and meter refinements for BUD3.
func registerBud3() {
	// Keep every budget figure on an opaque surface with the primary text token.
	// The figures remain outside the colored progress fill in both themes.
	rule(".bento-budgets .budget-card-loader",
		height("5px"),
		minHeight("5px"),
		position("relative"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-pill)"),
		overflow("visible"),
	)
	rule(".bento-budgets .budget-card-loader .bar-fill",
		height("3px"),
		borderRadius("var(--radius-pill)"),
	)
	rule(".bento-budgets .budget-card-loader-figs",
		position("relative"),
		display("flex"),
		justifyContent("space-between"),
		gap("0.75rem"),
		marginTop("0.45rem"),
		padding("0"),
		height("auto"),
		background("var(--bg-card)"),
		color("var(--text)"),
		fontSize("var(--type-14)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".bento-budgets .budget-card-loader-figs .budget-pct, .bento-budgets .budget-card-loader-figs .budget-spent, .bento-budgets .budget-card-loader-figs .budget-limit-btn",
		color("var(--text)"),
	)
	rule(".budget-limit-btn",
		minWidth("24px"),
		minHeight("24px"),
		padding("0.1rem 0.25rem"),
		borderRadius("var(--radius-sm)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		textDecoration("underline"),
		textUnderlineOffset("2px"),
	)
	rule(".budget-limit-editor",
		display("flex"),
		alignItems("end"),
		flexWrap("wrap"),
		gap("0.5rem 0.75rem"),
		marginTop("0.55rem"),
		padding("0.55rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"),
		background("var(--bg-card)"),
		color("var(--text)"),
	)
	rule(".budget-limit-editor-cap, .budget-limit-editor-base",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		fontSize("var(--type-13)"),
		color("var(--text)"),
	)
	rule(".budget-limit-editor-base > span",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".budget-limit-editor .budget-limit-input", width("7rem"))
	rule(".budget-limit-editor button", minHeight("24px"), minWidth("24px"))
	rule(".budget button, .budget-crow button, .budget-limit-editor button",
		minHeight("24px"),
		minWidth("24px"),
	)
	rule(".bento-budgets .budget-limit-editform",
		flex("1 1 100%"),
		flexWrap("wrap"),
		position("relative"),
		background("var(--bg-card)"),
		color("var(--text)"),
	)
	rule(".budget-limit-edit-values",
		display("flex"),
		gap("0.75rem"),
	)
	rule(".budget-strip-label",
		fontSize("var(--type-14)"),
		fontWeight("500"),
		letterSpacing("normal"),
		textTransform("none"),
		color("var(--text)"),
	)
	rule(".budget-crow-head",
		flex("1 1 0"),
		minWidth("0"),
	)
	rule(".budget-crow-name",
		flex("1 1 0"),
		minWidth("0"),
	)
	rule(".budget-crow .budget-crow-amt, .budget-crow .budget-crow-left",
		color("var(--text)"),
		whiteSpace("nowrap"),
	)
	rule(".budget-crow .budget-crow-left.is-over, .budget-crow .budget-crow-left.text-down",
		color("var(--text)"),
		background("var(--bg-card)"),
	)
	rule(".budget-crow-chip",
		minHeight("24px"),
		padding("0.2rem 0.45rem"),
		color("var(--text)"),
		background("var(--bg-elev)"),
	)
	rule(".budget-followup-count",
		minWidth("24px"),
		minHeight("24px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		borderRadius("var(--radius-pill)"),
		padding("0 0.4rem"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		fontSize("var(--type-13)"),
		fontWeight("600"),
	)
	rule(".budget-pace-legend",
		fontSize("var(--type-13)"),
		color("var(--text)"),
		marginTop("0.35rem"),
	)
	// The compact layout retains the amount, remaining/over text, and status at all
	// widths; the secondary meter drops out when it cannot carry useful information.
	ruleContentMax(860-railCollapsedPx, ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) minmax(6.5rem, auto) auto"),
		prop("grid-template-areas", `"name amount actions" "left status status"`),
		rowGap("0.15rem"),
	)
	ruleContentMax(860-railCollapsedPx, ".budget-crow-head", gridArea("name"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-bar", display("none"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-amt", gridArea("amount"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-left", display("block"), gridArea("left"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-chip", gridArea("status"), justifySelf("start"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-actions", gridArea("actions"))
	ruleMedia("(pointer: coarse)",
		".budget-crow button, .budget-limit-btn, .budget-limit-editor button, .budget-todos-head, .budget-goalglyph",
		minWidth("44px"), minHeight("44px"),
	)
}
