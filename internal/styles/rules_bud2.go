// SPDX-License-Identifier: MIT

package styles

// registerBud2Surface emits the compact Budgets status strip and responsive
// toolbar rules. Register it last so the focused page refinements win the cascade.
func registerBud2Surface() {
	rule(".bud2-status-details", marginTop("0.35rem"))
	rule(".bud2-status-details > summary",
		minHeight("32px"),
		padding("0.35rem 0.25rem"),
		color("var(--muted)"),
		cursor("pointer"),
		fontSize("var(--type-13)"),
	)
	rule(".bud2-member-lens",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		minWidth("0"),
	)
	rule(".bud2-member-share",
		margin("0.25rem 0 0"),
		fontSize("var(--type-12)"),
		color("var(--muted)"),
	)
	rule(".bud2-tools-menu",
		maxHeight("min(70vh, 42rem)"),
		overflowY("auto"),
		prop("overscroll-behavior", "contain"),
	)
	rule(".sample-banner",
		flexShrink("0"),
		maxWidth("none"),
	)
	ruleMedia("(max-width: 640px)", ".budgets-tb .filter-toolbar-actions",
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
	)
	ruleMedia("(max-width: 640px)", "main.cf-scroll",
		prop("padding-bottom", "calc(7.5rem + env(safe-area-inset-bottom, 0px))"),
	)
}
