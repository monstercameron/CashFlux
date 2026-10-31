// SPDX-License-Identifier: MIT

package styles

// registerTxnCalendar emits the /transactions calendar view (TX8): the month-grid
// day cells (net figure + transaction-density dots + recurring ghost labels) and
// the prev/next month nav. It reuses the shared .cal-grid/.cal-cell/.cal-head/
// .cal-dot vocabulary from the bills calendar and layers the ledger-specific
// pieces. Token-based throughout so it tracks every theme.
func registerTxnCalendar() {
	// The loader sits over the rows and fades out, so the day's rows show beneath it at once.
	keyframes("txnCalPreviewLoaderOut",
		at("0%", prop("opacity", "1")),
		at("70%", prop("opacity", "1")),
		at("100%", prop("opacity", "0"), prop("visibility", "hidden")),
	)
	keyframes("txnCalPreviewContentIn",
		at("0%", prop("opacity", "0")),
		at("100%", prop("opacity", "1")),
	)
	rule(".txn-cal-preview-body",
		display("grid"),
		minWidth("14rem"),
	)
	rule(".txn-cal-preview-loading, .txn-cal-preview-content",
		gridArea("1 / 1"),
	)
	rule(".txn-cal-preview-loading",
		zIndex("1"),
		background("var(--bg-card)"),
		prop("animation", "txnCalPreviewLoaderOut 420ms ease-out both"),
	)
	rule(".txn-cal-preview-content",
		prop("animation", "txnCalPreviewContentIn 180ms ease-out 240ms both"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".txn-cal-preview-loading",
		display("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".txn-cal-preview-content",
		prop("animation", "none"),
	)
	// The day preview lists its rows in the popover. The loader sits beside its words.
	rule(".txn-cal-preview-list",
		display("grid"),
		gap("0.25rem"),
		minWidth("14rem"),
		maxWidth("22rem"),
	)
	rule(".txn-cal-preview-row",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.75rem"),
		fontSize("var(--fs-body)"),
	)
	rule(".txn-cal-preview-label",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".txn-cal-preview-amount",
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	rule(".txn-cal-preview-loading",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		color("var(--text-dim)"),
		fontSize("var(--fs-meta)"),
	)
	rule(".txn-cal",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
		prop("container-type", "inline-size"),
	)
	rule(".txn-cal-nav",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "0.5rem"),
	)
	rule(".txn-cal-scroll-hint",
		prop("display", "none"),
		prop("color", "var(--muted)"),
		prop("font-size", "var(--fs-meta)"),
		prop("text-align", "right"),
	)
	rawBlock(`@container (max-width: 960px){.txn-cal-scroll-hint{display:inline}}`)
	rule(".txn-cal-month",
		prop("font-weight", "600"),
		prop("font-size", "var(--fs-body)"),
		prop("min-width", "10rem"),
		prop("text-align", "center"),
	)
	rule(".txn-cal-viewport",
		prop("max-width", "100%"),
		prop("overflow-x", "auto"),
		prop("overscroll-behavior-inline", "contain"),
		prop("scrollbar-width", "thin"),
		prop("scrollbar-color", "var(--border) transparent"),
	)
	rule(".txn-cal-weeks",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(8,minmax(7rem,1fr))"),
		prop("min-width", "56rem"),
		prop("gap", "4px"),
	)
	rule(".txn-cal-week-head, .txn-cal-week-row",
		prop("display", "contents"),
	)
	rule(".txn-cal-week-head",
		prop("margin-top", ".5rem"),
	)
	rule(".txn-cal-week-head .cal-head",
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "500"),
		prop("color", "var(--muted)"),
		prop("text-transform", "none"),
	)
	rule(".txn-cal-week-total",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("justify-content", "center"),
		prop("gap", "4px"),
		prop("box-sizing", "border-box"),
		prop("min-width", "0"),
		prop("height", "8rem"),
		prop("overflow", "hidden"),
		prop("padding", "6px 8px"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-card)"),
	)
	rule(".txn-cal-week-spent, .txn-cal-week-income, .txn-cal-week-count",
		prop("font-size", "var(--fs-body)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-cal-week-count",
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "500"),
		prop("color", "var(--muted)"),
	)
	rule(".txn-cal-week-spent",
		prop("font-weight", "600"),
		prop("white-space", "nowrap"),
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
		prop("gap", "2px"),
		prop("box-sizing", "border-box"),
		prop("min-width", "0"),
		prop("height", "8rem"),
		prop("overflow", "hidden"),
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
		prop("cursor", "pointer"),
		prop("background", "transparent"),
	)
	rule(".txn-cal-cell.today",
		prop("border-color", "var(--accent)"),
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "-2px"),
		prop("background", "var(--bg-card)"),
		prop("box-shadow", "none"),
	)
	rule(".txn-cal-legend",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", ".4rem"),
	)
	rule(".txn-cal-legend-dots",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
	)
	rule(".txn-cal-legend-dot",
		prop("position", "static"),
		prop("width", "7px"),
		prop("height", "7px"),
		prop("border-radius", "50%"),
		prop("background", "var(--accent)"),
	)
	rule(".txn-cal-net",
		prop("font-size", "var(--fs-body)"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	// One line per figure keeps every day cell the same height, whatever its amounts say.
	rule(".txn-cal-moved, .txn-cal-week-moved",
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	// The day preview is a read-only hover card. It must not take the pointer, or the cell under
	// it would lose its hover and the card would close as it opened.
	rule(".cf-overlay__surface:has([data-testid=\"txn-cal-preview\"])",
		prop("pointer-events", "none"),
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
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "500"),
		prop("line-height", "1.25"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
		prop("border-left", "2px dashed color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("padding-left", "4px"),
	)
	rule(".txn-cal-ghost-status, .txn-cal-ghost-label",
		prop("display", "block"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-cal-ghost-missed",
		prop("white-space", "nowrap"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
	)
	rule(".txn-cal-more-due",
		prop("display", "block"),
		prop("font-size", "var(--fs-meta)"),
		prop("font-weight", "600"),
		prop("color", "var(--accent)"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-cal-overflow",
		prop("border-top", "1px solid var(--border)"),
		prop("padding-top", "0.5rem"),
		prop("font-size", "var(--fs-meta)"),
	)
	rule(".txn-cal-overflow summary",
		prop("width", "fit-content"),
		prop("cursor", "pointer"),
		prop("color", "var(--accent)"),
		prop("font-weight", "600"),
	)
	rule(".txn-cal-overflow summary:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	rule(".txn-cal-overflow-list",
		prop("display", "grid"),
		prop("gap", "0.5rem"),
		prop("padding", "0.65rem 0 0.25rem"),
	)
	rule(".txn-cal-overflow-day",
		prop("display", "flex"),
		prop("gap", "0.75rem"),
	)
	rule(".txn-cal-overflow-date",
		prop("min-width", "3.5rem"),
		prop("color", "var(--muted)"),
	)
	rule(".txn-cal-overflow-items",
		prop("display", "grid"),
		prop("gap", "0.4rem"),
	)
	rule(".txn-cal-overflow-item",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.25rem 0.5rem"),
	)
	rule(".txn-cal-ghost-missed",
		prop("color", "var(--warn, #d97706)"),
		prop("border-left-color", "var(--warn, #d97706)"),
	)
}
