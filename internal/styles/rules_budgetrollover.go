// SPDX-License-Identifier: MIT

package styles

// registerBudgetRolloverBadge styles the C395 per-row rollover policy badge: a small
// pill that reads "No rollover" (quiet/faint), "Rolls over", or "Rolls over · cap N"
// (accent-toned) and, on click, opens a deterministic carryover-math popover. The goal
// is policy that's legible at a glance without shouting over the row's real signals.
//
// Theme tokens only (--text-dim / --border / --accent), so light and dark both track.
// Chained from registerBudgetsSurface (not install.go, which is contended).
func registerBudgetRolloverBadge() {
	rule(".budget-rollover-badge",
		display("inline-flex"),
		alignItems("center"),
	)
	rule(".budget-rollover-pill",
		prop("appearance", "none"),
		fontFamily("inherit"),
		cursor("pointer"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		fontSize("var(--type-11)"),
		fontWeight("600"),
		lineHeight("1"),
		padding("0.15rem 0.45rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
	)
	rule(".budget-rollover-pill:hover",
		borderColor("var(--text-dim)"),
	)
	// Off: the common case, kept deliberately quiet so a "No rollover" pill on most
	// rows never competes with the row's real signals.
	rule(".budget-rollover-badge.is-off .budget-rollover-pill",
		color("var(--text-dim)"),
		fontWeight("500"),
		opacity("0.7"),
	)
	// On / capped: accent-toned so an active rollover policy is the thing that stands out.
	rule(".budget-rollover-badge.is-on .budget-rollover-pill,\n  .budget-rollover-badge.is-capped .budget-rollover-pill",
		color("var(--accent)"),
		borderColor("color-mix(in srgb, var(--accent) 40%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
}
