// SPDX-License-Identifier: MIT

package styles

// registerBudgetsRecurringSurface emits the "Recurring in your budgets" tile
// (feedback #5): the committed-per-month header, the list of recurring rows with
// their frequency pill and amount, and the manage footer. Token-based so it
// tracks both themes. Registered from Register().
func registerBudgetsRecurringSurface() {
	rule(".brc-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("margin-bottom", "0.15rem"),
	)
	rule(".brc-total-label",
		prop("font-size", "var(--type-12)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.03em"),
		prop("margin-right", "0.4rem"),
	)
	rule(".brc-total-val", prop("font-size", "1.35rem"), prop("font-weight", "700"))
	rule(".brc-count", prop("font-size", "var(--type-12)"))

	rule(".brc-rows",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.6rem"),
	)
	rule(".brc-row",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("padding", "0.55rem 0.6rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("background", "var(--bg-card)"),
	)
	// Frequency pill — the point of the feature: how often, at a glance.
	rule(".brc-cadence",
		prop("flex", "0 0 auto"),
		prop("font-size", "0.72rem"),
		prop("font-weight", "700"),
		prop("letter-spacing", "0.02em"),
		prop("padding", "0.15rem 0.55rem"),
		prop("border-radius", "999px"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
		prop("color", "var(--accent)"),
		prop("white-space", "nowrap"),
		prop("min-width", "5.5rem"),
		prop("text-align", "center"),
	)
	rule(".brc-body", prop("flex", "1 1 auto"), prop("min-width", "0"))
	rule(".brc-label",
		prop("display", "block"),
		prop("font-weight", "600"),
		prop("color", "var(--text)"),
		prop("white-space", "nowrap"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
	)
	rule(".brc-meta", prop("margin-top", "0.05rem"))
	rule(".brc-amtcol",
		prop("flex", "0 0 auto"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("text-align", "right"),
	)
	rule(".brc-amt", prop("font-weight", "700"), prop("color", "var(--text)"))
	rule(".brc-permo", prop("font-size", "var(--type-12)"))
	rule(".brc-foot", prop("margin-top", "0.75rem"))
}
