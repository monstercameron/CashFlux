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

	// ── Precedence chain: first-match-wins as a numbered spine. ─────────────
	rule(".rule-chain",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("margin-top", "0.6rem"),
		prop("border-left", "2px solid var(--border)"),
		prop("margin-left", "0.85rem"),
	)
	rule(".rule-chain-item",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.85rem"),
		prop("padding", "0.45rem 0 0.45rem 1rem"),
		prop("position", "relative"),
	)
	// The precedence number sits ON the spine as an accent-ringed disc.
	rule(".rule-chain-n",
		prop("position", "absolute"),
		prop("left", "-0.85rem"),
		prop("top", "50%"),
		prop("transform", "translateY(-50%)"),
		prop("width", "1.7rem"),
		prop("height", "1.7rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("border-radius", "50%"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--accent)"),
		prop("color", "var(--accent)"),
		prop("font-size", "0.85rem"),
		prop("font-weight", "700"),
	)
	rule(".rule-chain-body",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "baseline"),
		prop("gap", "0.25rem 0.6rem"),
		prop("padding-left", "1rem"),
	)
	rule(".rule-chain-match",
		prop("font-weight", "600"),
	)
	rule(".rule-chain-cat",
		prop("color", "var(--text-dim)"),
		prop("font-size", "0.85rem"),
	)
	rule(".rule-chain-warn",
		prop("font-size", "0.78rem"),
		prop("flex-basis", "100%"),
	)
	rule(".rule-chain-item.rule-chain-shadowed .rule-chain-match",
		prop("opacity", "0.55"),
		prop("text-decoration", "line-through"),
	)
	rule(".rule-chain-item.rule-chain-shadowed .rule-chain-n",
		prop("border-color", "var(--border)"),
		prop("color", "var(--text-dim)"),
	)
}
