// SPDX-License-Identifier: MIT

package styles

// registerUxbatch3 emits the chrome for UX review batch 3 (tasks #16, #17):
//   - the collapsed goal card's COMPACT saved-vs-set-aside legend, and the legend
//     "saved" swatch tints that sample the bar's actual status color (so a Watch
//     amber bar shows an amber swatch, not a hardcoded accent-green one);
//   - the /health factor "value meter": a gauge whose LENGTH encodes the value on a
//     sensible 0..1 scale with a small tick at the target, colored by status — so an
//     on-target card no longer draws a full bar regardless of value.
//
// All colors use theme tokens (var(--text)/(--accent)/(--up)/(--danger)); the amber
// warn tone mirrors the app's existing #f59e0b bar tint. The status-tinted swatch
// rules carry an extra class over the base .goal-legend-swatch.is-saved rule in
// rules_goals.go, so they win on specificity regardless of registration order.
func registerUxbatch3() {
	const track = "color-mix(in srgb, var(--text) 9%, transparent)"

	// --- Task #16: compact legend on the collapsed goal card -----------------------
	// Tighter than the expanded legend (no reassurance note), sitting right under the bar.
	rule(".bento-goals .goal-legend-compact",
		margin("0.35rem 0 0.15rem"),
		fontSize("var(--type-12)"),
		gap("0.15rem 0.8rem"),
	)

	// The legend "saved" swatch samples the bar's ACTUAL saved-segment color: a
	// status-tinted bar (Watch=amber, At-risk/overdue=red, complete=green) gets a
	// matching swatch instead of the hardcoded accent-green default. Values mirror the
	// .bar-fill.<state> rules in rules_gen.go so the two never drift.
	rule(".goal-legend-swatch.is-saved.soon",
		background("#f59e0b"),
	)
	rule(".goal-legend-swatch.is-saved.final",
		background("linear-gradient(90deg, var(--accent), var(--up, #54b884))"),
	)
	rule(".goal-legend-swatch.is-saved.atrisk",
		background("var(--danger)"),
	)
	rule(".goal-legend-swatch.is-saved.overdue",
		background("var(--danger)"),
	)
	rule(".goal-legend-swatch.is-saved.done",
		background("var(--up, #54b884)"),
	)

	// --- Task #17: /health factor value meter --------------------------------------
	// The bar LENGTH is the value on a per-factor scale; a tick marks the target. The
	// fill color is the factor's status tone (good/warn/bad), so length and color carry
	// different, complementary facts (how far along vs. how healthy).
	rule(".hlt-meter",
		position("relative"),
		height("8px"),
		borderRadius("var(--radius-pill)"),
		overflow("visible"),
		background(track),
		margin("0.5rem 0 0.35rem"),
	)
	rule(".hlt-meter-fill",
		position("absolute"),
		top("0"),
		left("0"),
		height("100%"),
		borderRadius("var(--radius-pill)"),
		transition("width 0.25s ease"),
	)
	rule(".hlt-meter-fill.is-good",
		background("var(--up, #54b884)"),
	)
	rule(".hlt-meter-fill.is-warn",
		background("#f59e0b"),
	)
	rule(".hlt-meter-fill.is-bad",
		background("var(--danger)"),
	)
	// The target marker: a slim vertical tick that overhangs the track top and bottom
	// so it reads as a reference line, not part of the fill.
	rule(".hlt-meter-tick",
		position("absolute"),
		top("-2px"),
		width("2px"),
		height("12px"),
		marginLeft("-1px"),
		borderRadius("1px"),
		background("var(--text)"),
		opacity("0.55"),
	)
}
