// SPDX-License-Identifier: MIT

package styles

// registerAccountsSurface emits the accounts-page refinements: the merged editor's
// "update value" group and the readable, expandable notes line on each account row.
// Registered from Register().
func registerAccountsSurface() {
	// --- merged editor: the "update value / balance" group at the top of the form ---
	// A subtly framed panel so the marquee account action (record a new value) reads as
	// its own section, distinct from the account-detail fields below it.
	rule(".acct-value-section",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("padding", "0.8rem 0.9rem"),
		prop("margin-bottom", "0.9rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "10px"),
		prop("background", "var(--hover)"),
	)
	// The "Currently $X" context sits just under the input; the delta preview under that.
	rule(".acct-value-now",
		prop("font-size", "0.8rem"),
	)
	rule(".acct-value-delta",
		prop("margin", "0.05rem 0 0"),
	)

	// --- account row: readable, expandable notes line ---
	// The attached note itself, shown as a subtle framed disclosure that clamps to two
	// lines and expands on click — legible at a glance, not hidden in a hover tooltip.
	rule(".acct-notes",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("gap", "0.4rem"),
		prop("width", "100%"),
		prop("max-width", "42rem"),
		prop("text-align", "left"),
		prop("margin-top", "0.4rem"),
		prop("padding", "0.4rem 0.55rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "8px"),
		prop("background", "transparent"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "0.82rem"),
		prop("line-height", "1.4"),
		prop("cursor", "pointer"),
		prop("transition", "background .15s ease, border-color .15s ease, color .15s ease"),
	)
	rule(".acct-notes:hover",
		prop("background", "var(--hover)"),
		prop("border-color", "var(--text-dim)"),
		prop("color", "var(--text)"),
	)
	rule(".acct-notes:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	rule(".acct-notes-icon",
		prop("margin-top", "0.12rem"),
		prop("opacity", "0.75"),
	)
	// Collapsed: clamp to two lines with an ellipsis. Preserves the note's own line
	// breaks (white-space:pre-wrap) so a multi-line note still reads naturally.
	rule(".acct-notes-text",
		prop("display", "-webkit-box"),
		prop("-webkit-line-clamp", "2"),
		prop("-webkit-box-orient", "vertical"),
		prop("overflow", "hidden"),
		prop("white-space", "pre-wrap"),
		prop("word-break", "break-word"),
	)
	// Expanded: reveal the whole note.
	rule(".acct-notes.open .acct-notes-text",
		prop("-webkit-line-clamp", "unset"),
		prop("overflow", "visible"),
	)

	// --- AC1: account groups (sections + subtotal header) ---
	rule(".acct-group",
		prop("margin-bottom", "0.6rem"),
	)
	// A quiet section header above each group's rows: name on the left, net subtotal on
	// the right, with a subtle rule underneath so the grouping reads without shouting.
	rule(".acct-group-header",
		prop("padding", "0.35rem 0.15rem 0.3rem"),
		prop("margin-top", "0.2rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("font-size", "0.85rem"),
	)
	rule(".acct-group-name",
		prop("color", "var(--text)"),
	)
	rule(".acct-group-subtotal",
		prop("font-variant-numeric", "tabular-nums"),
	)

	// --- AC2: balance sparkline ---
	// A compact trend line under the account meta; muted so it supports, not competes.
	rule(".acct-spark",
		prop("display", "block"),
		prop("width", "120px"),
		prop("height", "24px"),
		prop("margin-top", "0.3rem"),
		prop("opacity", "0.85"),
		prop("overflow", "visible"),
	)

	// --- AC9: in / out / net flow figures ---
	rule(".acct-flow",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.15rem"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("margin-top", "0.2rem"),
	)
	rule(".acct-flow-net",
		prop("font-weight", "500"),
	)
}
