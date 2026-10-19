// SPDX-License-Identifier: MIT

package styles

// registerUXT6 quiets the Review intro and keeps the filter sheet to one scroll area.
func registerUXT6() {
	rule(".rvs-intro", marginBottom("1.1rem"))
	rule(".rvs-intro-title",
		margin("0 0 .25rem"),
		fontFamily("var(--font-sans), sans-serif"),
		fontSize("var(--type-17)"),
		fontWeight("600"),
		lineHeight("1.25"),
	)
	rule(".rvs-intro-count",
		margin("0"),
		color("var(--text-dim)"),
		fontSize("var(--type-14)"),
		lineHeight("1.4"),
	)
	rule(".rvs-dot.is-look, .rvs-tier.is-look .rvs-tier-mark",
		background("var(--text-faint)"),
	)
	rule(".rvs-grp.is-focus .rvs-grp-row",
		outline("2px solid var(--accent)"),
		outlineOffset("-2px"),
	)
	rule(".filter-inline-panel.filter-sheet",
		overflowY("auto"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-inline-body",
		flex("0 0 auto"),
		minHeight("auto"),
		overflow("visible"),
	)
}
