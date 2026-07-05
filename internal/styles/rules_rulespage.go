// SPDX-License-Identifier: MIT

package styles

// registerRulesSurface emits the /rules automation-ledger design: the auto-row
// bento host and the per-rule weight column (match count over a share bar of
// the heaviest rule). Hero/section/takeaway chrome reuses the shared
// rpt-*/debt-* rules so the page reads as a sibling of the Understand
// surfaces. Registered from Register().
func registerRulesSurface() {
	rule(".bento.bento-rules",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-rules > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	rule(".bento.bento-rules > .w:has(.add-menu:not(.hidden-menu))",
		prop("z-index", "30"),
	)
	// The rule's weight column: how many transactions its phrase catches.
	rule(".rule-figure",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.1rem"),
		prop("min-width", "8rem"),
		prop("flex-shrink", "0"),
	)
	rule(".rule-figure-n",
		prop("font-size", "0.95rem"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rule-figure-sub",
		prop("font-size", "0.7rem"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	rule(".bento-rules .row-main .share-bar",
		prop("max-width", "24rem"),
	)
	// The match phrase is the rule's identity — give it the display voice.
	rule(".bento-rules .row-desc .rule-match",
		prop("font-weight", "600"),
	)
}
