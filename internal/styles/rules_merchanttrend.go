// SPDX-License-Identifier: MIT

package styles

// registerMerchantTrendSurface emits the transactions-row spending-trend affordance
// (TX6b): the small trend chip on a row, the popover that holds the merchant story,
// and the loading spinner shown for the 500ms lazy-compute beat. Registered after
// registerGenerated so the `.mtrend-pop` overrides win over the shared `.add-menu`
// chrome it reuses for positioning/edge-flip. Theme tokens only.
func registerMerchantTrendSurface() {
	// The chip: a quiet, small trending glyph that sits after the row's badges. Low-key
	// at rest, accent on hover/focus, so it reads as "there's history here" without
	// shouting on every row.
	rule(".mtrend-wrap",
		marginLeft("0.35rem"),
	)
	rule(".mtrend-chip",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("24px"),
		height("24px"),
		padding("0"),
		border("1px solid transparent"),
		borderRadius("6px"),
		background("transparent"),
		color("color-mix(in srgb, var(--text) 55%, transparent)"),
		cursor("pointer"),
		transition("color 0.12s ease, background 0.12s ease, border-color 0.12s ease"),
	)
	rule(".mtrend-chip:hover, .mtrend-chip[aria-expanded=\"true\"]",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent) 35%, var(--border))"),
	)

	// The popover reuses .add-menu (absolute + AnchorPopover edge-flip) but holds a small
	// card, not a button list — so widen it and give it comfortable padding + block flow.
	rule(".mtrend-pop",
		display("block"),
		minWidth("240px"),
		maxWidth("300px"),
		// Reserve roughly the loaded card's height so the box does NOT grow when the
		// spinner swaps to content — the edge-flip is measured once on open, so a stable
		// size keeps a bottom/right-edge popover from overflowing after it loads.
		minHeight("148px"),
		padding("0.75rem 0.85rem"),
	)
	rule(".mtrend-card",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	// The this-vs-typical delta: a real figure, colour-coded — over-usual draws the eye
	// (danger), under/at-usual stays quiet. Never the uppercase `.badge` pill.
	rule(".mtrend-delta",
		fontSize("0.78rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	rule(".mtrend-delta.is-up",
		color("var(--danger, #d8716f)"),
	)
	rule(".mtrend-delta.is-down, .mtrend-delta.is-flat",
		color("color-mix(in srgb, var(--text) 58%, transparent)"),
	)
	// The trend card's own sparkline stretches to the card width for a readable curve.
	rule(".mtrend-card svg",
		width("100%"),
		height("34px"),
	)

	// Loading beat: a centered spinner while the merchant's stats compute (~500ms).
	rule(".mtrend-loading",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		minHeight("128px"),
	)
	rule(".mtrend-spinner",
		width("20px"),
		height("20px"),
		borderRadius("50%"),
		border("2px solid color-mix(in srgb, var(--text) 15%, transparent)"),
		borderTopColor("var(--accent)"),
		animation("boot-spin 0.7s linear infinite"),
	)
	// Respect reduced-motion: the spinner becomes a calm pulse rather than a spin.
	ruleMedia("(prefers-reduced-motion: reduce)", ".mtrend-spinner",
		animation("none"),
		opacity("0.6"),
	)

	// --- Sparkline scale metadata (so the line is readable, not a bare squiggle) ----
	rule(".mtrend-spark",
		display("flex"),
		flexDirection("column"),
		gap("2px"),
	)
	rule(".mtrend-spark-row",
		display("flex"),
		alignItems("stretch"),
		gap("0.5rem"),
	)
	// The line stretches to fill; override the generic `.mtrend-card svg` 100% width so
	// the y-axis legend keeps its column.
	rule(".mtrend-card .mtrend-spark-row svg",
		flex("1 1 auto"),
		minWidth("0"),
		width("auto"),
		height("42px"),
	)
	// y-axis legend: the highest charge at the top, the lowest at the bottom.
	rule(".mtrend-spark-yaxis",
		display("flex"),
		flexDirection("column"),
		justifyContent("space-between"),
		flexShrink("0"),
		textAlign("right"),
		fontSize("0.68rem"),
		fontVariantNumeric("tabular-nums"),
		color("color-mix(in srgb, var(--text) 58%, transparent)"),
	)
	// x-axis: the time span the line covers (oldest → newest charge).
	rule(".mtrend-spark-xaxis",
		display("flex"),
		justifyContent("space-between"),
		fontSize("0.66rem"),
		fontVariantNumeric("tabular-nums"),
		color("color-mix(in srgb, var(--text) 52%, transparent)"),
	)
	rule(".mtrend-spark-meta",
		fontSize("0.68rem"),
		color("color-mix(in srgb, var(--text) 62%, transparent)"),
		marginTop("1px"),
	)
}
