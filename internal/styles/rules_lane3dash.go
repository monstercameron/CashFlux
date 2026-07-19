// SPDX-License-Identifier: MIT

package styles

// registerLane3Dashboard holds the 2026-07-19 dashboard lane-3 refinements
// (C366 / C414 / C415): explaining the Focus view picker, the Monthly-recap
// compact-width label behavior, distinct Needs-attention severity tiers, and a
// calmer edit-layout mode. Registered after registerGenerated() (and after the
// earlier dashboard passes) so these equal-specificity refinements win the
// cascade. Theme tokens only, so everything tracks light/dark.
func registerLane3Dashboard() {
	// --- C366: Focus view picker — a live description + standing subtitle. ------
	// The picker sits in the hero actions row (a wrapping flex row). It becomes its
	// own small column: the control pill on top, then the description lines. Top-
	// align the row so the taller picker doesn't stretch the sibling buttons.
	rule(".home-hero-actions",
		alignItems("flex-start"),
	)
	rule(".dash-preset-picker",
		display("inline-flex"),
		flexDirection("column"),
		gap("0.25rem"),
	)
	rule(".dash-preset-desc",
		fontSize("0.72rem"),
		lineHeight("1.3"),
		color("var(--text-dim)"),
		maxWidth("22rem"),
	)
	rule(".dash-preset-sub",
		fontSize("0.66rem"),
		lineHeight("1.25"),
		color("var(--text-faint)"),
		maxWidth("22rem"),
	)

	// --- C414(a): Monthly-recap labels at compact desktop width. ---------------
	// Wide tiles keep the elegant one-line ellipsis + hover tooltip. But once the
	// content pane narrows the recap cells shrink and a long category name /
	// biggest-expense description clips to "Gro…". Rather than widening, let the
	// value, sub-line, and label WRAP so the full text reads across two lines, and
	// tighten the inter-cell spacing so each cell gets a little more room. Keyed on
	// CONTENT width (pane minus rail), not the viewport, per the app's breakpoints.
	ruleContentMax(contentGrid4, ".cf-recap-val",
		whiteSpace("normal"),
		overflow("visible"),
		textOverflow("clip"),
		overflowWrap("anywhere"),
	)
	ruleContentMax(contentGrid4, ".cf-recap-sub",
		whiteSpace("normal"),
		overflow("visible"),
		textOverflow("clip"),
		overflowWrap("anywhere"),
	)
	ruleContentMax(contentGrid4, ".cf-recap-lbl",
		whiteSpace("normal"),
		overflowWrap("anywhere"),
	)
	ruleContentMax(contentGrid4, ".cf-recap-stats",
		gap("0.85rem"),
	)
	ruleContentMax(contentGrid4, ".cf-recap-stat + .cf-recap-stat",
		paddingLeft("0.85rem"),
	)

	// --- C414(b): distinct Needs-attention severity tiers. ---------------------
	// attention.Rank scores Critical / Warning / Info, but the rows read equally
	// weighted (in dark mode all three were a plain outlined pill differing only by
	// a dot color / left bar). Give each tier distinct WEIGHT so urgency scans at a
	// glance: Critical = heaviest (a danger-tinted fill + bolder text), Warning =
	// medium (a faint amber fill), Info = lightest (muted text, no fill). The
	// glyphs already differ (alert triangle / ● / ○). Backgrounds are dark-mode
	// additive — the light theme keeps its own tuned tints (higher-specificity
	// [data-theme="light"] rules in the generated sheet win there); the bolder
	// Critical weight and the muted Info tone apply in both themes.
	rule(".attention-item.is-critical",
		background("color-mix(in srgb, var(--danger) 12%, transparent)"),
		fontWeight("600"),
	)
	rule(".attention-item.is-warning",
		background("color-mix(in srgb, var(--warn) 8%, transparent)"),
	)
	rule(".attention-item.is-info",
		color("var(--text-dim)"),
	)

	// --- C415: edit-mode calm. -------------------------------------------------
	// In edit-layout mode every tile showed its drag grip and settings gear at once
	// — a wall of chrome that made rearranging feel busy. Keep the uniform dashed
	// tile outline (registered elsewhere) as the single "these move" cue, but hide
	// the per-tile grip + gear until the user is actually working a tile. Resize
	// handles already hover/focus-reveal (base .rz is opacity:0); this extends the
	// same discipline to the grip and gear. Reveal happens on:
	//   - :hover           — the tile the mouse is over,
	//   - :focus-within    — the tile a keyboard user has tabbed into (focus-visible
	//                        on the tile or any handle satisfies this), and
	//   - [aria-grabbed]   — the tile a keyboard user has grabbed (Space/Enter) to
	//                        move or resize.
	// So keyboard users always retain access; only the idle tiles go quiet.
	rule(`.bento[data-layout-edit="on"] .w .grip`,
		opacity("0"),
		pointerEvents("none"),
	)
	rule(`.bento[data-layout-edit="on"] .w .gear-inline`,
		opacity("0"),
		pointerEvents("none"),
	)
	rule(`.bento[data-layout-edit="on"] .w:hover .grip, .bento[data-layout-edit="on"] .w:focus-within .grip, .bento[data-layout-edit="on"] .w[aria-grabbed="true"] .grip`,
		opacity("1"),
		pointerEvents("auto"),
	)
	rule(`.bento[data-layout-edit="on"] .w:hover .gear-inline, .bento[data-layout-edit="on"] .w:focus-within .gear-inline, .bento[data-layout-edit="on"] .w[aria-grabbed="true"] .gear-inline`,
		opacity("1"),
		pointerEvents("auto"),
	)
	// The grabbed tile also surfaces its resize handles (base .rz only reveals them
	// on hover / focus-within / focus-visible — a keyboard grab is none of those).
	rule(`.bento[data-layout-edit="on"] .w[aria-grabbed="true"] .rz`,
		opacity(".9"),
		pointerEvents("auto"),
	)
}
