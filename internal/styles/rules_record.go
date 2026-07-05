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
	// The aside holds actor + (on the newest row) Undo — keep them apart.
	rule(".bento-record .row-aside",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("flex-shrink", "0"),
	)
}
