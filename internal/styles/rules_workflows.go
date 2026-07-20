// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerWorkflowsSurface emits the bespoke /workflows surface — a
// from-scratch "automations desk", NOT the shared card kit: a masthead, the
// three savings quick-start panels as one band, then the automation registry
// (ledger rows with status dots and dry-run-first controls) beside the
// composer whose footprint reads the draft back in plain English. Registered
// after the generated rules so these win equal-specificity ties.
func registerWorkflowsSurface() {
	const mono = "ui-monospace, SFMono-Regular, Menlo, monospace"

	rule(".wf-deck",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.75rem"),
		prop("margin-top", "1rem"),
	)

	// ── Section language: serif accent-tick titles, quiet ledes. ────────────────
	rule(".wf-sec-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.6rem"),
		prop("margin-bottom", "0.35rem"),
	)
	rule(".wf-sec-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "var(--type-20)"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
		prop("margin", "0"),
	)
	rule(".wf-sec-lede",
		prop("font-size", "var(--type-14)"),
		prop("opacity", "0.65"),
		prop("margin", "0.35rem 0 1rem"),
		prop("max-width", "46rem"),
	)
	rule(".wf-count",
		prop("font-family", mono),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--accent)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 35%, transparent)"),
		prop("border-radius", "var(--radius-pill)"),
		prop("padding", "0 0.45rem"),
		prop("line-height", "1.35"),
	)

	// ── Savings quick-starts: three template panels in one band. ────────────────
	rule(".wf-quick-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(3, minmax(0, 1fr))"),
		prop("gap", "1.5rem"),
		prop("align-items", "start"),
	)
	ruleContentMax(contentTwoCol, ".wf-quick-grid",
		prop("grid-template-columns", "minmax(0, 1fr)"),
	)
	rule(".wf-quick-panel",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "14px"),
		prop("padding", "1.1rem 1.2rem"),
		prop("background", "color-mix(in srgb, var(--text) 2.5%, transparent)"),
	)
	rule(".wf-panel-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.05rem"),
		prop("font-weight", "600"),
		prop("margin", "0 0 0.25rem"),
	)
	rule(".wf-panel-desc",
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.5"),
		prop("opacity", "0.65"),
		prop("margin", "0 0 0.9rem"),
	)
	rule(".wf-panel-enable",
		prop("margin", "0 0 0.85rem"),
		prop("font-size", "var(--type-14)"),
	)
	rule(".wf-quick-panel .field",
		prop("width", "100%"),
	)
	rule(".wf-panel-save",
		prop("align-self", "flex-start"),
		prop("margin-top", "0.35rem"),
	)
	// The "Already running" summary inside a quick-start panel.
	rule(".wf-active",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.35rem"),
		prop("border-left", "3px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("padding", "0.2rem 0 0.2rem 0.8rem"),
		prop("margin-bottom", "0.9rem"),
	)
	rule(".wf-active-line",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("font-size", "0.84rem"),
	)
	rule(".wf-active-line.is-off",
		prop("opacity", "0.55"),
	)
	rule(".wf-dot.is-off",
		prop("background", "none"),
		prop("border", "1.5px solid var(--border)"),
		prop("box-shadow", "none"),
	)
	// The raw formula shown as the auditable aside under a translated footprint.
	rule(".wf-foot-raw",
		prop("font-family", mono),
		prop("font-size", "var(--type-11)"),
		prop("opacity", "0.55"),
		prop("margin-top", "0.1rem"),
	)
	rule(".wf-quick-panel .ok",
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--accent)"),
		prop("margin", "0 0 0.5rem"),
	)
	rule(".wf-quick-panel .err",
		prop("font-size", "var(--type-13)"),
		prop("margin", "0 0 0.5rem"),
	)

	// ── The desk: registry + history beside the composer rail. ──────────────────
	rule(".wf-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 25rem"),
		prop("gap", "2.5rem"),
		prop("align-items", "start"),
	)
	ruleContentMax(contentTwoCol, ".wf-grid",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".wf-main",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("min-width", "0"),
	)
	rule(".wf-empty",
		prop("font-size", "0.88rem"),
		prop("opacity", "0.6"),
		prop("margin", "0.5rem 0 0"),
	)

	// Registry rows.
	rule(".wf-row",
		prop("padding", "0.85rem 0.2rem"),
		prop("border-bottom", "1px solid color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".wf-row-head",
		prop("display", "grid"),
		prop("grid-template-columns", "auto minmax(0, 1fr) auto"),
		prop("gap", "0.8rem"),
		prop("align-items", "center"),
	)
	rule(".wf-dot",
		prop("width", "9px"),
		prop("height", "9px"),
		prop("border-radius", "var(--radius-pill)"),
		prop("background", "var(--accent)"),
		prop("box-shadow", "0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent)"),
	)
	rule(".wf-row.is-off .wf-dot",
		prop("background", "none"),
		prop("border", "1.5px solid var(--border)"),
		prop("box-shadow", "none"),
	)
	rule(".wf-row.is-off .wf-row-main",
		prop("opacity", "0.55"),
	)
	rule(".wf-row-top",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
	)
	rule(".wf-name",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.05rem"),
		prop("font-weight", "600"),
	)
	rule(".wf-meta",
		prop("font-size", "var(--type-12)"),
		prop("opacity", "0.62"),
		prop("margin-top", "0.1rem"),
	)
	rule(".wf-cond",
		prop("font-family", mono),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--accent)"),
	)
	rule(".wf-row-actions",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".wf-dry",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		prop("color", "var(--accent)"),
	)

	// Run results: an accent-ticked read-out under the row.
	rule(".wf-result",
		prop("margin", "0.6rem 0 0 1.55rem"),
		prop("border-left", "3px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("padding", "0.15rem 0 0.15rem 0.8rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.2rem"),
	)
	rule(".wf-result-head",
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.1em"),
		prop("text-transform", "uppercase"),
		prop("opacity", "0.55"),
	)
	rule(".wf-result-line",
		prop("font-size", "var(--type-13)"),
		prop("opacity", "0.8"),
	)
	rule(".wf-result-err",
		prop("margin", "0.5rem 0 0 1.55rem"),
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--danger)"),
	)
	rule(".wf-cond-warn",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--danger)"),
		prop("margin", "-0.5rem 0 0.75rem"),
	)
	rule(".wf-result-quiet",
		prop("margin", "0.5rem 0 0 1.55rem"),
		prop("font-size", "var(--type-13)"),
		prop("opacity", "0.55"),
	)

	// Run history: quiet mono-stamped hairline rows.
	rule(".wf-history",
		prop("margin-top", "1.75rem"),
	)
	rule(".wf-hist-row",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("padding", "0.45rem 0.2rem"),
		prop("border-bottom", "1px solid color-mix(in srgb, var(--text) 6%, transparent)"),
	)
	rule(".wf-hist-name",
		prop("font-size", "var(--type-14)"),
		prop("font-weight", "600"),
	)
	rule(".wf-hist-meta",
		prop("font-family", mono),
		prop("font-size", "var(--type-12)"),
		prop("opacity", "0.55"),
		prop("white-space", "nowrap"),
	)

	// ── Composer rail: the fld-composer margin-note language. ───────────────────
	rule(".wf-composer",
		prop("border-left", "1px solid var(--border)"),
		prop("padding-left", "1.75rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	ruleContentMax(contentTwoCol, ".wf-composer",
		prop("border-left", "none"),
		prop("padding-left", "0"),
		prop("max-width", "34rem"),
	)
	rule(".wf-comp-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "var(--type-18)"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
		prop("margin", "0"),
	)
	rule(".wf-comp-lede",
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.5"),
		prop("opacity", "0.65"),
		prop("margin", "0.4rem 0 1rem"),
	)
	rule(".wf-composer .field",
		prop("width", "100%"),
	)
	rule(".wf-cond-help",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("margin", "-0.35rem 0 0.9rem"),
	)
	rule(".wf-hint",
		prop("font-size", "var(--type-12)"),
		prop("opacity", "0.55"),
		prop("line-height", "1.45"),
		prop("margin", "0"),
	)
	rule(".wf-varselect",
		prop("max-width", "16rem"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".wf-actions-head",
		prop("border-top", "1px solid var(--border)"),
		prop("padding-top", "0.85rem"),
		prop("margin-bottom", "0.6rem"),
	)
	rule(".wf-param",
		prop("margin-bottom", "0.6rem"),
	)
	rule(".wf-param .field",
		prop("width", "100%"),
	)
	rule(".wf-addaction",
		prop("align-self", "flex-start"),
		prop("margin-bottom", "0.85rem"),
	)
	rule(".wf-staged",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("margin-bottom", "0.85rem"),
	)
	rule(".wf-staged .row",
		prop("padding", "0.35rem 0"),
		prop("border-bottom", "1px solid color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".wf-save",
		prop("align-self", "stretch"),
		prop("margin-top", "0.85rem"),
	)
}
