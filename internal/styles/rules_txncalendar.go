// SPDX-License-Identifier: MIT

package styles

// registerTxnCalendar emits the /transactions calendar view (TX8): the month-grid
// day cells (net figure + transaction-density dots + recurring ghost labels) and
// the prev/next month nav. It reuses the shared .cal-grid/.cal-cell/.cal-head/
// .cal-dot vocabulary from the bills calendar and layers the ledger-specific
// pieces. Token-based throughout so it tracks every theme.
func registerTxnCalendar() {
	rule(".txn-cal",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
	)
	rule(".txn-cal-nav",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
	)
	rule(".txn-cal-month",
		prop("font-weight", "600"),
		prop("font-size", "1rem"),
		prop("min-width", "10rem"),
		prop("text-align", "center"),
	)
	rule(".txn-cal-weeks",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(7,minmax(0,1fr)) 8rem"),
		prop("gap", "4px"),
	)
	rule(".txn-cal-week-head, .txn-cal-week-row",
		prop("display", "contents"),
	)
	rule(".txn-cal-week-head",
		prop("margin-top", ".5rem"),
	)
	rule(".txn-cal-week-head .cal-head",
		prop("font-size", "12px"),
		prop("font-weight", "500"),
		prop("color", "var(--muted)"),
		prop("text-transform", "none"),
	)
	rule(".txn-cal-week-total",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("justify-content", "center"),
		prop("gap", "4px"),
		prop("min-height", "84px"),
		prop("padding", "6px 8px"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-card)"),
	)
	rule(".txn-cal-week-spent, .txn-cal-week-income, .txn-cal-week-count",
		prop("font-size", "var(--type-13)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".txn-cal-week-count",
		prop("font-size", "12px"),
		prop("font-weight", "500"),
		prop("color", "var(--muted)"),
	)
	rule(".txn-cal-week-spent",
		prop("font-weight", "600"),
	)
	rule(".txn-cal-week-income",
		prop("font-weight", "500"),
	)
	// Day cells are buttons: reset the button chrome so they read as calendar cells,
	// keep them keyboard-focusable, and give a clear focus ring.
	rule(".txn-cal-cell",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "stretch"),
		prop("gap", "3px"),
		prop("min-height", "84px"),
		prop("padding", "6px 8px"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-card)"),
		prop("text-align", "left"),
		prop("cursor", "pointer"),
		prop("transition", "background 0.12s ease, border-color 0.12s ease"),
	)
	rule(".txn-cal-cell:hover:not(:disabled)",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 8%, var(--bg-card))"),
	)
	rule(".txn-cal-cell:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "-2px"),
	)
	rule(".txn-cal-cell.out",
		prop("opacity", "0.4"),
		prop("cursor", "default"),
		prop("background", "transparent"),
	)
	rule(".txn-cal-cell.today",
		prop("border-color", "var(--accent)"),
		prop("box-shadow", "inset 0 0 0 1px var(--accent)"),
	)
	rule(".txn-cal-net",
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".txn-cal-net.text-down",
		prop("color", "var(--money-negative)"),
	)
	rule(".txn-cal-dots",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "3px"),
		prop("margin-top", "auto"),
	)
	rule(".txn-cal-dots .cal-dot",
		prop("width", "5px"),
		prop("height", "5px"),
		prop("border-radius", "50%"),
		prop("background", "color-mix(in srgb, var(--accent) 70%, var(--border))"),
	)
	// Ghost: a dimmed recurring label projected onto its due date; no interaction.
	rule(".txn-cal-ghost",
		prop("display", "block"),
		prop("font-size", "12px"),
		prop("font-weight", "500"),
		prop("line-height", "1.25"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
		prop("border-left", "2px dashed color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("padding-left", "4px"),
	)
	rule(".txn-cal-ghost-missed",
		prop("color", "var(--warn, #d97706)"),
		prop("border-left-color", "var(--warn, #d97706)"),
	)
}
