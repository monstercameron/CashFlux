// SPDX-License-Identifier: MIT

package styles

// registerHealthSurface emits the /health bento-surface design: the host grid,
// the factor-tile value/variable-chip chrome, and the formula-identity block.
// Section/hero/takeaway chrome reuses the shared debt-*/rpt-* rules; everything
// is token-based so it tracks every theme. Registered from Register().
func registerHealthSurface() {
	rule(".bento.bento-health",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-health > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Focus-next step rows: buttons stretch + center their text by default inside
	// the tile — pin the step copy left so the list reads as rows.
	rule(".bento-health .row",
		prop("align-items", "flex-start"),
		prop("text-align", "left"),
	)
	// Factor tile: the current value vs its target on one line above the meter.
	rule(".hlt-factor-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("margin-bottom", "0.4rem"),
	)
	rule(".hlt-factor-value",
		prop("font-size", "1.6rem"),
		prop("font-weight", "700"),
		prop("line-height", "1.05"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// The factor's live engine-variable identity chip.
	rule(".hlt-varchip",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.25rem"),
		prop("margin-top", "0.6rem"),
		prop("padding", "0.15rem 0.55rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("font-size", "0.72rem"),
		prop("background", "color-mix(in srgb, var(--accent) 7%, transparent)"),
	)
	rule(".hlt-varchip code",
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("color", "var(--accent)"),
	)
	// Factor footer: the variable chip left, the act CTA right.
	rule(".hlt-factor-foot",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("margin-top", "0.6rem"),
	)
	rule(".hlt-factor-foot .hlt-varchip",
		prop("margin-top", "0"),
	)
	// The per-factor scoring-curve disclosure.
	rule(".hlt-curve > summary",
		prop("cursor", "pointer"),
		prop("font-size", "0.75rem"),
		prop("opacity", "0.7"),
		prop("margin-top", "0.35rem"),
	)
	// Composition / Scoring / Example blocks inside the disclosure, each with a small
	// uppercase label.
	rule(".hlt-detail",
		prop("margin-top", "0.7rem"),
	)
	rule(".hlt-detail-label",
		prop("display", "block"),
		prop("font-size", "0.64rem"),
		prop("font-weight", "700"),
		prop("letter-spacing", "0.05em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--text-faint)"),
		prop("margin-bottom", "0.15rem"),
	)
	// The factor's composition equation (variable = molecule / atoms).
	rule(".hlt-eq",
		prop("display", "block"),
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("font-size", "0.72rem"),
		prop("line-height", "1.5"),
		prop("color", "var(--text)"),
		prop("background", "color-mix(in srgb, var(--accent) 6%, var(--bg-elev))"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "6px"),
		prop("padding", "0.4rem 0.55rem"),
		prop("margin-top", "0.3rem"),
		prop("overflow-x", "auto"),
		prop("white-space", "pre-wrap"),
		prop("overflow-wrap", "anywhere"),
	)
	rule(".hlt-eq-note",
		prop("display", "block"),
		prop("font-size", "0.66rem"),
		prop("margin-top", "0.2rem"),
	)
	rule(".hlt-formula > summary",
		prop("cursor", "pointer"),
	)
	// The score's formula identity under the hero ring.
	rule(".hlt-formula",
		prop("margin-top", "1rem"),
		prop("padding-top", "0.85rem"),
		prop("border-top", "1px dashed var(--border)"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.35rem"),
	)
	rule(".hlt-formula-code",
		prop("display", "block"),
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("font-size", "0.78rem"),
		prop("line-height", "1.55"),
		prop("color", "var(--accent)"),
		prop("background", "var(--bg-elev)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "8px"),
		prop("padding", "0.55rem 0.75rem"),
		prop("overflow-x", "auto"),
		prop("white-space", "pre-wrap"),
		prop("word-break", "break-word"),
	)
}
