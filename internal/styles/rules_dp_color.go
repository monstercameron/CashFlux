// SPDX-License-Identifier: MIT

package styles

// registerDpColor tightens color SEMANTICS from the frontend-design review, using only
// color / background / border-color / fill properties (layout, radius, type and borders
// belong to other passes). Two jobs:
//
//  1. Green is reserved for the ONE beneficial primary action per region (Add
//     transaction, Add budget, Save, Contribute…). A handful of ADMINISTRATIVE /
//     neutral controls carry .btn-primary only because they're the primary control in
//     their region — a settings/preferences "Done", a config-modal "Done", a plain
//     "Close". Those read as beneficial money moves in green, so they're pinned to the
//     neutral button tone. Targeted by data-testid so the beneficial primaries that
//     share .btn-primary stay green.
//
//  2. An OVER-budget bar no longer saturates the whole track in solid red. The fill is
//     already capped at 100% of the track (the row caps width at 100), so a maxed bar
//     and a 3%-over bar looked identical — a flat red block. Instead the capped
//     over-fill gets a diagonal red hazard-hatch, so "over the limit" reads as an
//     overshoot texture distinct from a bar that merely reached its limit; the percent
//     figure inside the bar ("112%") carries the by-how-much.
//
// Theme tokens only (var(--bg-elev)/--text/--border for the neutral button, var(--danger)
// for the over-hatch), so light and dark both track. Registered LAST so these overrides
// win at equal specificity over the generated rules.
//
// (The goal "Watch" state was audited here too and is already amber end-to-end — the
// .pace-watch badge, the "soon" bar fill, and the is-soon card tint — so it needed no
// change; green never leaks into a Watch goal.)
func registerDpColor() {
	// --- 1. Administrative / dismissal buttons → neutral, not beneficial-green ---
	// Two attribute selectors' specificity (0,2,0) beats the bare .btn-primary (0,1,0)
	// and ties the .modal-foot/.modal-sticky-foot .btn-primary rules (also 0,2,0) — this
	// file registers last, so the tie breaks in its favor.
	neutralBtns := ".btn-primary[data-testid=\"subs-prefs-done\"],\n" +
		"  .btn-primary[data-testid=\"allocate-strategy-done\"],\n" +
		"  .btn-primary[data-testid=\"cover-all-close\"]"
	rule(neutralBtns,
		background("var(--bg-elev)"),
		color("var(--text)"),
		borderColor("var(--border)"),
	)
	// Hover must also stay neutral: the .modal-foot .btn-primary:hover green (0,3,0)
	// would otherwise repaint two of these on hover. Match its specificity (0,3,0) and
	// win on source order.
	neutralBtnsHover := ".btn-primary[data-testid=\"subs-prefs-done\"]:hover,\n" +
		"  .btn-primary[data-testid=\"allocate-strategy-done\"]:hover,\n" +
		"  .btn-primary[data-testid=\"cover-all-close\"]:hover"
	rule(neutralBtnsHover,
		background("var(--hover)"),
		color("var(--text)"),
		borderColor("var(--text-dim)"),
	)

	// --- 2. Over-budget bar: cap the red, hatch the overshoot ---
	// Same specificity (0,3,0) as the generated .bento-budgets .bar-fill.over; this file
	// registers last, so the diagonal hazard-hatch replaces the solid danger fill. Covers
	// both the card loader and the compact crow bar (both live under .bento-budgets).
	rule(".bento-budgets .bar-fill.over",
		background("repeating-linear-gradient(45deg,"+
			" var(--danger) 0, var(--danger) 7px,"+
			" color-mix(in srgb, var(--danger) 60%, #000) 7px,"+
			" color-mix(in srgb, var(--danger) 60%, #000) 14px)"),
	)
}
