// SPDX-License-Identifier: MIT

package styles

// registerTV7C keeps calendar text on the shared type scale and restores AA
// contrast for the day numbers used to pad adjacent months.
func registerTV7C() {
	rule(".txn-cal-cell.out",
		prop("opacity", "1"),
	)
	rule(".txn-cal-cell.out .cal-day",
		prop("color", "var(--muted)"),
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "500"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".txn-cal-week-moved, .txn-cal-moved, .txn-cal-ghost-status, .txn-cal-ghost-label",
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "500"),
		prop("font-variant-numeric", "tabular-nums"),
	)
}
