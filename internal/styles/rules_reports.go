// SPDX-License-Identifier: MIT

package styles

// registerReportsSurface emits the /reports bento-surface design: the host grid,
// the hero (net figure + delta + figure chips), the takeaway pull-quotes, the
// share bars, the chart pair, the heads-up alert tone, and the scope-filter
// chips (previously completely unstyled). Token-based throughout
// (var(--accent/--bg-elev/--border/…)) so it tracks every theme. Registered
// from Register() after the main sheet (a separate file so the surface owns
// its rules).
func registerReportsSurface() {
	rule(".bento.bento-reports",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-reports > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// A tile with an OPEN dropdown must paint above its later siblings — tiles
	// are transformed (own stacking contexts), so without this the next tile's
	// chrome swallows clicks on the export menu hanging over it.
	rule(".bento.bento-reports > .w:has(.add-menu:not(.hidden-menu))",
		prop("z-index", "30"),
	)

	// ── Hero: eyebrow (period coverage), the net figure + delta, figure chips. ──
	rule(".rpt-hero",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.85rem"),
	)
	rule(".rpt-hero-eyebrow",
		prop("margin", "0"),
		prop("font-size", "var(--type-12)"),
	)
	rule(".rpt-hero-main",
		prop("display", "flex"),
		prop("align-items", "flex-end"),
		prop("justify-content", "space-between"),
		prop("flex-wrap", "wrap"),
		prop("gap", "1.25rem"),
	)
	rule(".rpt-hero-label",
		prop("font-size", "var(--type-12)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.08em"),
	)
	rule(".rpt-hero-value",
		prop("font-size", "2.6rem"),
		prop("font-weight", "700"),
		prop("line-height", "1.05"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rpt-delta",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.3rem"),
		prop("margin-top", "0.35rem"),
		prop("padding", "0.1rem 0.55rem"),
		prop("border-radius", "var(--radius-pill)"),
		prop("font-size", "0.75rem"),
		prop("border", "1px solid var(--border)"),
		prop("background", "var(--bg-elev)"),
	)
	rule(".rpt-delta.pos", prop("color", "var(--up, #54b884)"))
	rule(".rpt-delta.neg", prop("color", "var(--down, #d8716f)"))
	rule(".rpt-chip-sub",
		prop("font-size", "var(--type-12)"),
		prop("margin-top", "0.15rem"),
	)
	rule(".rpt-hero-trend", prop("margin", "0"))

	// ── Takeaway pull-quote: the one-sentence insight leading a section. ────────
	rule(".rpt-takeaway",
		prop("margin", "0 0 0.6rem"),
		prop("font-size", "var(--type-16)"),
		prop("font-style", "italic"),
		prop("border-left", "2px solid var(--accent)"),
		prop("padding-left", "0.7rem"),
	)

	// ── Ranked-row share bars (class-based; previously inline styles). ──────────
	rule(".share-bar",
		prop("height", "8px"),
		prop("max-width", "100%"),
		prop("margin-top", "0.3rem"),
		prop("background", "var(--border)"),
		prop("border-radius", "var(--radius-pill)"),
		prop("overflow", "hidden"),
	)
	rule(".share-bar-thin", prop("height", "4px"))
	rule(".share-bar-fill",
		prop("height", "100%"),
		prop("background", "var(--accent)"),
		prop("border-radius", "var(--radius-pill)"),
	)

	// Ranked-row drill buttons: buttons stretch + center by default inside the
	// flex row — pin them left so the category names read as a list.
	rule(".bento-reports .row-desc.btn-link",
		prop("align-self", "flex-start"),
		prop("text-align", "left"),
	)
	// The custom-field grouper select shouldn't stretch the full tile width.
	rule(".bento-reports #cf-field-select",
		prop("max-width", "16rem"),
	)
	// "New this period" category tag: neutral metadata chrome (red stays
	// reserved for negative money).
	rule(".rpt-new-tag",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-pill)"),
		prop("padding", "0 0.45rem"),
		prop("font-size", "var(--type-11)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
	)
	// Zeroed-category disclosure: the categories that had spending last period
	// but none now, folded behind a quiet summary line.
	rule(".rpt-zeroed > summary",
		prop("cursor", "pointer"),
		prop("font-size", "var(--type-13)"),
		prop("margin", "0.6rem 0 0.3rem"),
		prop("opacity", "0.75"),
	)

	// ── Category card: the bar + donut pair side-by-side on wide screens. ───────
	rule(".rpt-chart-pair",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0,1fr) minmax(0,1fr)"),
		prop("gap", "1rem"),
		prop("align-items", "start"),
		prop("margin-bottom", "0.75rem"),
	)
	ruleContentMax(contentTwoCol, ".rpt-chart-pair",
		prop("grid-template-columns", "1fr"),
	)

	// ── Heads-up anomaly tile: a quiet danger left-accent. ──────────────────────
	rule(".rpt-headsup",
		prop("border-left", "3px solid var(--down, #d8716f)"),
		prop("padding-left", "0.85rem"),
	)

	// ── Scope filter (the #444 chips): previously had no CSS at all. ────────────
	rule(".scope-selector",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.55rem"),
		prop("margin-top", "0.85rem"),
		prop("padding-top", "0.85rem"),
		prop("border-top", "1px dashed var(--border)"),
	)
	rule(".scope-row",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
	)
	rule(".scope-label",
		prop("flex", "0 0 7.5rem"),
		prop("font-size", "var(--type-12)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		prop("color", "var(--text-dim)"),
		prop("opacity", "0.7"),
	)
	rule(".scope-chips",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.35rem"),
	)
	rule(".scope-chip",
		prop("appearance", "none"),
		prop("border", "1px solid var(--border)"),
		prop("background", "transparent"),
		prop("color", "inherit"),
		prop("border-radius", "var(--radius-pill)"),
		prop("padding", "0.2rem 0.7rem"),
		prop("font-size", "var(--type-12)"),
		prop("line-height", "1.4"),
		prop("cursor", "pointer"),
		prop("transition", "border-color 120ms ease, background 120ms ease"),
	)
	rule(".scope-chip:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)
	rule(".scope-chip-on",
		prop("border-color", "var(--accent)"),
		prop("background", "color-mix(in srgb, var(--accent) 16%, transparent)"),
	)
	rule(".scope-chip-clear",
		prop("border-style", "dashed"),
	)
	rule(".scope-accts",
		// 2026-07-17 audit: a household with many accounts turned the scope panel
		// into a wall of chips — cap the chip well and scroll inside it.
		prop("max-height", "7.5rem"),
		prop("overflow-y", "auto"),
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.35rem 1rem"),
		prop("padding", "0.35rem 0 0.1rem"),
		prop("flex-basis", "100%"),
	)
	rule(".scope-acct-row",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("font-size", "var(--type-13)"),
		prop("cursor", "pointer"),
	)
	rule(".scope-sv-select",
		prop("max-width", "14rem"),
	)
	rule(".scope-save-form",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
	)
}
