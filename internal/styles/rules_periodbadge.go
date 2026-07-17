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
}
