// SPDX-License-Identifier: MIT

package styles

// registerLane3Dashboard holds the 2026-07-19 dashboard lane-3 refinements
// (C366 / C414 / C415): explaining the Focus view picker, the Monthly-recap
// compact-width label behavior, distinct Needs-attention severity tiers, and a
// calmer edit-layout mode. Registered after registerGenerated() (and after the
// earlier dashboard passes) so these equal-specificity refinements win the
// cascade. Theme tokens only, so everything tracks light/dark.
func registerLane3Dashboard() {
	// --- C366: Focus view picker — a live description + standing subtitle. ------
	// The picker sits in the hero actions row (a wrapping flex row). It becomes its
	// own small column: the control pill on top, then the description lines. Top-
	// align the row so the taller picker doesn't stretch the sibling buttons.
	rule(".home-hero-actions",
		alignItems("flex-start"),
	)
	rule(".dash-preset-picker",
		display("inline-flex"),
		flexDirection("column"),
		gap("0.25rem"),
	)
	rule(".dash-preset-desc",
		fontSize("0.72rem"),
		lineHeight("1.3"),
		color("var(--text-dim)"),
		maxWidth("22rem"),
	)
	rule(".dash-preset-sub",
		fontSize("0.66rem"),
		lineHeight("1.25"),
		color("var(--text-faint)"),
		maxWidth("22rem"),
	)
}
