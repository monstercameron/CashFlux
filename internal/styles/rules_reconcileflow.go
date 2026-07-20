// SPDX-License-Identifier: MIT

package styles

// registerReconcileFlow emits the guided reconcile-to-statement workflow
// refinements (2026-07-19 lane C): a sticky math header that keeps the
// statement inputs and the live remaining-difference readout anchored while the
// transaction list scrolls, the standing "what finishing requires" copy, and
// the "Mark all cleared" bulk row with its outcome preview.
//
// Chained from registerAccountsSurface (the reconcile editor is an accounts
// surface), so install.go is untouched by this lane.
func registerReconcileFlow() {
	// The math header pins to the top of the modal's scroll region. It needs an
	// opaque background so scrolling transaction rows never show through, and a
	// hairline rule to separate it from the list below.
	rule(".reconcile-header",
		prop("position", "sticky"),
		prop("top", "0"),
		prop("z-index", "2"),
		prop("background", "var(--bg-card)"),
		prop("padding-bottom", "0.6rem"),
		prop("margin-bottom", "0.5rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	// The cleared-balance + remaining-difference summary line under the inputs.
	rule(".reconcile-summary",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "baseline"),
		prop("gap", "0.35rem 1.25rem"),
		prop("margin-top", "0.55rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// The remaining-difference figure is the anchor number — bolder than the
	// quiet cleared-balance figure beside it.
	rule(".reconcile-remaining",
		prop("font-weight", "600"),
	)
	// When the difference reaches zero the readout goes accent-toned to signal
	// "ready to finish".
	rule(".reconcile-remaining.is-zero",
		prop("color", "var(--accent)"),
	)
	// Standing explanation of what finishing requires — a calm caption, never
	// shouting.
	rule(".reconcile-explain",
		prop("margin", "0.55rem 0 0"),
	)
	// The "Mark all cleared" bulk row: the button and its outcome preview sit on
	// one line, wrapping gracefully on a narrow modal.
	rule(".reconcile-markall",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("margin", "0.65rem 0 0.25rem"),
	)
	rule(".reconcile-markall-preview",
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
	)
}
