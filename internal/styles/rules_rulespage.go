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

	// The flip-modal editors' Save/Cancel bar sticks to the bottom of the
	// scrollable body, so a tall form (e.g. a rule with all three condition
	// slots open) never hides its actions below an unsignposted scroll.
	rule(".dataedit-actions",
		prop("position", "sticky"),
		prop("bottom", "0"),
		prop("display", "flex"),
		prop("gap", "0.5rem"),
		prop("padding", "0.6rem 0 0.1rem"),
		prop("margin-top", "0.25rem"),
		prop("background", "var(--bg-card)"),
		prop("border-top", "1px solid var(--border)"),
	)

	// Condition-slot headers: breathe between the checkbox and its label
	// (they rendered flush — "☐Condition 1").
	rule(".cond-slot-header",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.45rem"),
	)

	// Block-level form rows span the whole grid. .form-grid is an auto-fit
	// column grid, so without this the conditions fieldset, the apply-existing
	// checkbox label, and the submit row each collapsed into a single ~150px
	// column BESIDE the fields on the wide /rules quick-add — helper paragraphs
	// rendered as 10ch-wide towers (task #1). Generic on purpose: any form-grid
	// row tagged fg-span is a full-width row (matches the .goal-add convention).
	rule(".form-grid > .fg-span",
		prop("grid-column", "1 / -1"),
	)
	// The conditions fieldset reads as a quiet sub-section, not a browser-default
	// bordered box; its legend takes the small set-label voice.
	rule(".cond-slots",
		prop("border", "0"),
		prop("padding", "0"),
		prop("margin", "0.2rem 0 0"),
		prop("min-width", "0"),
	)
	rule(".cond-slots > legend",
		prop("padding", "0"),
		prop("font-size", "0.78rem"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("margin-bottom", "0.15rem"),
	)
	rule(".cond-slots > .muted",
		prop("margin", "0 0 0.4rem"),
	)
	// An enabled slot's editors sit in one tidy field/op/value row that wraps on
	// narrow widths instead of stacking full-width blocks.
	rule(".cond-slot-body",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(auto-fit, minmax(160px, 1fr))"),
		prop("gap", "0.4rem"),
		prop("margin", "0.3rem 0 0.5rem 1.55rem"),
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
