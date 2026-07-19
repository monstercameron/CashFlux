// SPDX-License-Identifier: MIT

package styles

// registerDpBorders is the 2026-07-19 "reduce border dependence + vertical rhythm"
// pass from the frontend-design review. The page hierarchy had grown into boxes
// within boxes — an elevated widget/section, a bordered card inside it, a bordered
// stat/summary tile inside that — so nesting read busy. The refinement principle:
//
//	elevation for the LARGEST section, a border OR a state accent-edge for state,
//	and dividers/background for repeated inner content — never all three at once
//	on the same object.
//
// So a card/tile that already sits inside an elevated `.w` widget (or an already
// bordered `.card`) loses its own full border and leans on a one-step-up background
// instead. Backgrounds do the separating because `--bg-elev` sits above `--bg-card`
// in BOTH themes (dark: #1a1a1d over #121214; light: #efede8 inset from #fff), so the
// inner object still reads without a box drawn around it. The app already uses the
// `color-mix(var(--bg-elev) 48%, transparent)` subtle-lift idiom for inner cards
// (debt/budget/goal surfaces) — these rules extend it and drop the redundant border.
//
// Border/padding/margin/gap/background/box-shadow only; theme tokens only, so light
// and dark track automatically. Registered LAST in install.go so these win at equal
// specificity over the generated base rules.
func registerDpBorders() {
	// --- Doubled box #1: summary/stat tiles inside a bordered card or elevated widget.
	// The base `.stat` is `var(--bg-card)` + a full border; inside a `.card` (also
	// bg-card) or a `.w` (bg-card/white) that's border-on-border with no background
	// contrast. Drop the border and lift the tile with the subtle-elevation idiom, and
	// unify the inner padding to the row band (~13/16px) so the tile — not its box —
	// sets the rhythm. `.bento-debt .stat` already used this background; now it drops
	// its border too, so the whole app's stat tiles read the same calm way.
	rule(".card .stat, .w .stat",
		border("none"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		padding("0.8rem 1rem"),
	)

	// --- Doubled box #2: a `.card` dropped inside an elevated `.w` widget. Two full
	// boxes stacked. Drop the inner card's border AND its resting shadow (the parent
	// `.w` already provides the elevation), and lift with background so it reads as one
	// panel instead of a box-in-a-box in either theme.
	rule(".w .card",
		border("none"),
		boxShadow("none"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
	)

	// --- All three at once: budget & goal category cards. Each already carried the
	// parent `.w`'s elevation, a subtle card background, AND a stateful left accent-edge
	// (green / amber "at risk" / red "over") — plus a neutral full border on top. That's
	// elevation + accent-edge + border on one object. Keep the accent-edge (it's the
	// signal that carries state) and the background; remove the redundant neutral border.
	rule(".bento-budgets .budget, .bento-goals .goal-card",
		border("none"),
	)
	// The budgets surface re-asserts a top border on the first card; neutralize it so the
	// no-border treatment is consistent across the list.
	rule(".bento-budgets .budget:first-child",
		borderTop("none"),
	)
}
