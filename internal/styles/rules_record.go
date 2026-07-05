// SPDX-License-Identifier: MIT

package styles

// registerRecordSurface emits the /activity change-record design: the auto-row
// bento host, the serif day dividers, and the per-entry action ticks (accent
// for additions, danger for deletions, neutral for edits). Hero/section/
// takeaway chrome reuses the shared rpt-*/debt-* rules so the page reads as a
// sibling of the Understand surfaces. Registered from Register().
func registerRecordSurface() {
	rule(".bento.bento-record",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-record > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// A day divider: a small serif date with an accent tick, ruling the entries
	// beneath it into one visual day.
	rule(".act-day",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.55rem"),
		prop("margin", "1.1rem 0 0.2rem"),
		prop("font-size", "0.85rem"),
		prop("font-weight", "600"),
	)
	rule(".act-day::before",
		prop("content", "\"\""),
		prop("display", "block"),
		prop("width", "3px"),
		prop("height", "0.9rem"),
		prop("background", "var(--accent)"),
		prop("border-radius", "2px"),
	)
	rule(".act-day:first-child", prop("margin-top", "0.25rem"))
	// Entry rows carry a quiet action tick on the left edge.
	rule(".row.act-entry",
		prop("border-left", "3px solid var(--border)"),
		prop("padding-left", "0.75rem"),
	)
	rule(".row.act-entry.act-add",
		prop("border-left-color", "color-mix(in srgb, var(--accent) 65%, transparent)"),
	)
	rule(".row.act-entry.act-del",
		prop("border-left-color", "var(--down, #d8716f)"),
	)
	// The entity-filter select shouldn't stretch the full tile width.
	rule(".bento-record .act-filter",
		prop("max-width", "14rem"),
	)
	// Field-level before → after detail lines under an update entry: the field
	// name in a fixed quiet column, the old value struck through, the new value
	// carrying the weight.
	rule(".act-diff",
		prop("margin-top", "0.35rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
	)
	rule(".act-diff-line",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
		prop("flex-wrap", "wrap"),
		prop("font-size", "0.78rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".act-diff-field",
		prop("color", "var(--text-dim)"),
		prop("min-width", "6.5rem"),
	)
	rule(".act-diff-before",
		prop("color", "var(--text-dim)"),
		prop("text-decoration", "line-through"),
	)
	rule(".act-diff-arrow",
		prop("color", "var(--text-faint)"),
	)
	rule(".act-diff-after",
		prop("font-weight", "600"),
	)

	// The aside holds actor + (on the newest row) Undo — keep them apart.
	rule(".bento-record .row-aside",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("flex-shrink", "0"),
	)
}
