// SPDX-License-Identifier: MIT

package styles

// registerR3K keeps the transaction bulk actions over the main pane and reserves
// scroll room so its final row can still be reached.
func registerR3K() {
	rule(`html .w[data-widget="txn-bulkbar"]`,
		prop("inset", "0"),
		prop("width", "auto"),
		prop("height", "auto"),
		prop("pointer-events", "none"),
		prop("overflow", "visible"),
	)
	rule(`html .w[data-widget="txn-bulkbar"] .wbody-nohead`,
		prop("height", "0"),
		prop("padding", "0"),
		prop("overflow", "visible"),
	)
	rule(".txn-bulkbar-overlay",
		prop("position", "absolute"),
		prop("left", "max(256px, calc(50vw - 360px))"),
		prop("bottom", "16px"),
		prop("width", "min(960px, calc(100vw - 240px - 32px))"),
		prop("max-width", "960px"),
		prop("pointer-events", "auto"),
		prop("box-sizing", "border-box"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("min-height", "48px"),
		prop("padding", "0 1rem"),
		prop("overflow", "visible"),
		prop("flex-wrap", "wrap"),
		prop("white-space", "normal"),
		prop("background", "var(--bg-card)"),
		prop("border-top", "1px solid var(--border)"),
		prop("box-shadow", "0 -4px 18px rgba(0,0,0,.18)"),
	)
	// The shell mirrors the live rail state onto html.cf-rail-c. Deriving the
	// overlay's left edge and available width from that state keeps its center
	// aligned with #main without relying on a transformed tile or container query.
	rule(":where(html.cf-rail-c) .txn-bulkbar-overlay",
		prop("left", "max(74px, calc(50vw - 451px))"),
		prop("width", "min(960px, calc(100vw - 58px - 32px))"),
	)
	rule(".txn-bulkbar-overlay .bulk-tag-editor",
		prop("position", "absolute"),
		prop("right", "10rem"),
		prop("bottom", "calc(100% + 8px)"),
		prop("z-index", "72"),
		prop("padding", "0.5rem"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("box-shadow", "0 8px 24px rgba(0,0,0,.24)"),
	)
	rule(".bulk-picker", prop("position", "relative"), prop("flex", "0 1 auto"))
	rule(".bulk-picker-trigger",
		prop("max-width", "12rem"), prop("overflow", "hidden"), prop("text-overflow", "ellipsis"),
	)
	rule(".bulk-picker-popover",
		prop("position", "absolute"), prop("left", "0"), prop("bottom", "calc(100% + 8px)"),
		prop("z-index", "72"), prop("width", "min(20rem, calc(100vw - 2rem))"),
		prop("padding", "0.5rem"), prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"), prop("border-radius", "var(--radius-lg)"),
		prop("box-shadow", "0 8px 24px rgba(0,0,0,.24)"),
	)
	rule(".bulk-picker-search", prop("width", "100%"), prop("margin-bottom", "0.4rem"))
	rule(".bulk-picker-options",
		prop("display", "flex"), prop("flex-direction", "column"),
		prop("max-height", "16rem"), prop("overflow-y", "auto"),
	)
	rule(".bulk-picker-option",
		prop("width", "100%"), prop("text-align", "left"), prop("padding", "0.45rem 0.6rem"),
		prop("border", "0"), prop("border-radius", "var(--radius-sm)"),
		prop("background", "transparent"), prop("color", "var(--text)"), prop("cursor", "pointer"),
	)
	rule(".bulk-picker-option[aria-selected=true], .bulk-picker-option:hover", prop("background", "var(--bg-elev)"))
	rule(".bulk-picker-empty", prop("padding", "0.45rem 0.6rem"), prop("color", "var(--muted)"))
	rule("main.cf-scroll:has(.txn-has-selection)", prop("padding-bottom", "80px"))
}
