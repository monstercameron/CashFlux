// SPDX-License-Identifier: MIT

package styles

// registerBudgetOverCalm calms the C396 over-budget bar treatment. The OVER-budget
// bar used to carry a busy full-width diagonal hazard-hatch (the frontend-design
// review called it "more aggressive than necessary"). The fill now goes flat danger,
// and a THIN 12px overflow marker at the capped right edge carries the "past the
// limit" signal instead. The whole-bar texture was the aggression; a flat tone plus
// an edge marker is calmer and still unambiguous (the "112%" figure inside the bar
// carries the by-how-much).
//
// Theme tokens only (--danger). The override uses a HIGHER-specificity selector than
// the generated / dp-color rule (an extra ancestor class → 0,4,0 vs 0,3,0), so it
// wins regardless of registration order rather than depending on being registered
// last. Chained from registerBudgetsSurface (not install.go, which is contended).
func registerBudgetOverCalm() {
	// Flat danger fill — no diagonal hatch. NB: never set `position` here; the card /
	// crow fill is already absolutely positioned, and the marker below anchors to it.
	rule(".bento-budgets .budget-card-loader .bar-fill.over,\n  .bento-budgets .budget-crow-bar .bar-fill.over",
		background("var(--danger)"),
	)
	// The thin overflow marker: a slim hazard cap confined to the last 12px of the
	// capped fill (the bar caps at 100% width, so its right edge is the track's edge).
	// This replaces the full-bar diagonal hatch — the same "overshoot" cue at a
	// fraction of the visual weight. Pure ::after decoration (no pointer target).
	rule(".bento-budgets .budget-card-loader .bar-fill.over::after,\n  .bento-budgets .budget-crow-bar .bar-fill.over::after",
		content("\"\""),
		position("absolute"),
		top("0"),
		right("0"),
		bottom("0"),
		width("12px"),
		pointerEvents("none"),
		prop("background", "repeating-linear-gradient(45deg,"+
			" color-mix(in srgb, #000 26%, transparent) 0,"+
			" color-mix(in srgb, #000 26%, transparent) 2px,"+
			" transparent 2px, transparent 5px)"),
	)
}
