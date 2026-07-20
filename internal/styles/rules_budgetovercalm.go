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

	// --- Graded by overage magnitude (detail-lane 3) --------------------------------
	// The flat-fill + thin-cap treatment above is the MODERATE baseline (10–24% over).
	// A MILD overspend (<10% over) reads softer — the danger tone is pulled toward the
	// card surface and the overflow marker narrows — so a $3-over category doesn't shout
	// as loudly as a $300-over one. A SEVERE overrun (>=25% over) reads heavier — a
	// deeper danger tone and a wider, denser marker. The tier class (over-mild /
	// over-severe, from budgeting.OverTier) adds one class over the base `.over`
	// selector, so these win on specificity (0,5,0 vs 0,4,0) without order games.
	// Theme tokens only (--danger, --bg-card): both themes read.

	// MILD: soften the fill toward the card surface.
	rule(".bento-budgets .budget-card-loader .bar-fill.over.over-mild,\n  .bento-budgets .budget-crow-bar .bar-fill.over.over-mild",
		background("color-mix(in srgb, var(--danger) 68%, var(--bg-card))"),
	)
	// MILD marker: thinner (7px) and a lighter hatch, so the overshoot cue is present
	// but quiet.
	rule(".bento-budgets .budget-card-loader .bar-fill.over.over-mild::after,\n  .bento-budgets .budget-crow-bar .bar-fill.over.over-mild::after",
		width("7px"),
		prop("background", "repeating-linear-gradient(45deg,"+
			" color-mix(in srgb, #000 16%, transparent) 0,"+
			" color-mix(in srgb, #000 16%, transparent) 2px,"+
			" transparent 2px, transparent 6px)"),
	)

	// SEVERE: a deeper danger tone (mix a little black into --danger so it reads as the
	// heaviest state in both light and dark).
	rule(".bento-budgets .budget-card-loader .bar-fill.over.over-severe,\n  .bento-budgets .budget-crow-bar .bar-fill.over.over-severe",
		background("color-mix(in srgb, var(--danger) 88%, #000)"),
	)
	// SEVERE marker: wider (18px) and a denser hatch — the loudest overshoot cue.
	rule(".bento-budgets .budget-card-loader .bar-fill.over.over-severe::after,\n  .bento-budgets .budget-crow-bar .bar-fill.over.over-severe::after",
		width("18px"),
		prop("background", "repeating-linear-gradient(45deg,"+
			" color-mix(in srgb, #000 34%, transparent) 0,"+
			" color-mix(in srgb, #000 34%, transparent) 2px,"+
			" transparent 2px, transparent 4px)"),
	)

	// --- "Cover…" / "Top up…" must never truncate (detail-lane 3) --------------------
	// The card-footer money-move buttons carry short labels that were being clipped in
	// tighter layouts. Pin them to their content size so the label always fits: they
	// never shrink and never wrap mid-label, and min-width:fit-content guards against
	// any flex/grid context that would otherwise collapse them below the text. The
	// action row already wraps (flex-wrap), so a button that can't fit alongside its
	// neighbours drops to the next line WHOLE rather than losing its label.
	rule(".bento-budgets .budget-actions .btn",
		prop("flex", "0 0 auto"),
		prop("min-width", "fit-content"),
		whiteSpace("nowrap"),
	)
}
