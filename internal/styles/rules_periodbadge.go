// SPDX-License-Identifier: MIT

package styles

// registerPeriodBadge emits the widget-header "Today" chip: the small pill a
// current-state dashboard tile (net worth, bills, accounts, to-do, …) wears
// while the dashboard is paged to ANOTHER period, so a figure that ignores the
// selected month says so instead of silently reading as that month's value
// (the parity scan's dashboard period-contract defect). Theme tokens only.
func registerPeriodBadge() {
	rule(".w-today",
		display("inline-flex"),
		alignItems("center"),
		flexShrink("0"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text-dim)"),
		fontSize("0.62rem"),
		fontWeight("600"),
		letterSpacing("0.08em"),
		textTransform("uppercase"),
		whiteSpace("nowrap"),
	)

	// Top-bar "Updated Xm ago" stamp: a quiet text button in the context zone.
	rule(".tb-updated",
		background("none"),
		border("0"),
		padding("0.1rem 0.3rem"),
		borderRadius("6px"),
		color("var(--text-dim)"),
		font("inherit"),
		fontSize("0.78rem"),
		whiteSpace("nowrap"),
		cursor("pointer"),
	)
	rule(".tb-updated:hover, .tb-updated:focus-visible",
		color("var(--text)"),
		background("color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	// Crowded top bars (below the fold-into-More breakpoint) keep only the
	// history glyph; the title/aria-label still carry the full sentence.
	ruleMedia("(max-width: 1720px)", ".tb-updated .tb-updated-label",
		display("none"),
	)
	// Freshness card: stale-account chips are BUTTONS now (jump-to-account
	// repair affordance) — keep the member-chip look, add press affordances.
	rule(".fresh-chip",
		cursor("pointer"),
		font("inherit"),
	)
	rule(".fresh-chip:hover, .fresh-chip:focus-visible",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	// Dashboard drill affordances: widget rows/legend entries that route to
	// their filtered source keep their layout but read as quiet links.
	rule(".dash-drill",
		background("none"),
		border("0"),
		padding("0"),
		margin("0"),
		font("inherit"),
		color("inherit"),
		textAlign("inherit"),
		cursor("pointer"),
		borderRadius("6px"),
	)
	rule(".dash-drill:hover, .dash-drill:focus-visible",
		color("var(--accent)"),
	)
	rule(".dash-drill-row",
		cursor("pointer"),
	)
	rule(".dash-drill-row:hover td, .dash-drill-row:focus-visible td",
		color("var(--accent)"),
	)
	// Annual Review partial-period honesty + month drills (parity scan).
	rule(".rpta-partial-chip",
		display("inline-flex"),
		alignItems("center"),
		marginTop("0.35rem"),
		padding("0.15rem 0.6rem"),
		borderRadius("999px"),
		border("1px dashed var(--border)"),
		color("var(--text-dim)"),
		fontSize("0.78rem"),
	)
	rule(".rpta-month-drill",
		background("none"),
		border("0"),
		padding("0"),
		margin("0"),
		font("inherit"),
		color("inherit"),
		cursor("pointer"),
	)
	rule(".rpta-month-drill:hover, .rpta-month-drill:focus-visible",
		color("var(--accent)"),
		textDecoration("underline"),
		textUnderlineOffset("3px"),
	)
	rule(".rpta-inprogress",
		marginLeft("0.45rem"),
		padding("0.05rem 0.4rem"),
		borderRadius("999px"),
		border("1px dashed var(--border)"),
		color("var(--text-dim)"),
		fontSize("0.62rem"),
		fontWeight("600"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		whiteSpace("nowrap"),
	)
}
