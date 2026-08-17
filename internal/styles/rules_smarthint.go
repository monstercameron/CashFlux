// SPDX-License-Identifier: MIT

package styles

// registerSmartRowHint emits the shared item-scoped smart hint (SM-3, SM-5, SM-6,
// SM-8…SM-13, SM-16): a quiet inline line with a glyph, a sentence, an optional
// action, and an optional dismiss. Theme tokens only.
//
// It reads as an aside rather than as a record: a hairline left rule and a barely
// tinted ground, no card chrome. These sit INSIDE rows and cards that already have
// their own borders, and a second bordered box nested in the first is what makes a
// page feel cluttered rather than helpful.
func registerSmartRowHint() {
	rule(".smart-rowhint",
		display("flex"),
		alignItems("flex-start"),
		gap("0.5rem"),
		marginTop("0.4rem"),
		padding("0.4rem 0.55rem"),
		borderLeft("2px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		borderRadius("0 var(--radius-sm, 6px) var(--radius-sm, 6px) 0"),
		background("color-mix(in srgb, var(--accent) 7%, transparent)"),
		fontSize("var(--type-13)"),
		lineHeight("1.4"),
	)
	// The warn tone is for a hint that costs money if ignored (a fee bleed, an
	// overdraft forecast) — the same semantic red the figures use, not a new hue.
	rule(".smart-rowhint.is-warn",
		borderLeftColor("color-mix(in srgb, var(--danger) 60%, transparent)"),
		background("color-mix(in srgb, var(--danger) 7%, transparent)"),
	)
	rule(".smart-rowhint-body",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".smart-rowhint-text",
		color("var(--text)"),
		overflowWrap("anywhere"),
	)
	rule(".smart-rowhint-detail",
		marginTop("0.1rem"),
		overflowWrap("anywhere"),
	)
	// The action sits at the end of the line and stays on one line: these labels
	// are two or three words, and letting them wrap turns the aside into a block.
	rule(".smart-rowhint-act",
		flexShrink("0"),
		alignSelf("center"),
		whiteSpace("nowrap"),
	)
	rule(".smart-rowhint-x",
		flexShrink("0"),
		alignSelf("center"),
		color("var(--text-faint)"),
		cursor("pointer"),
	)
	rule(".smart-rowhint-x:hover",
		color("var(--text)"),
	)
	// On a narrow viewport the action drops under the sentence rather than
	// squeezing it to a two-word column.
	ruleMedia("(max-width:560px)", ".smart-rowhint",
		flexWrap("wrap"),
	)
	ruleMedia("(max-width:560px)", ".smart-rowhint-body",
		flex("1 1 100%"),
	)

	// --- SM-4: the "why this went over" sentence -------------------------------
	// It leads the budget card's drivers disclosure, so it is styled as the lede of
	// that panel rather than as another aside: no tint, no rule, just a slightly
	// stronger line above the merchant list it explains.
	rule(".budget-whyover",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		marginBottom("0.5rem"),
	)
	rule(".budget-whyover-row",
		display("flex"),
		alignItems("flex-start"),
		gap("0.4rem"),
	)
	rule(".budget-whyover-text",
		color("var(--text)"),
		fontSize("var(--type-13)"),
		lineHeight("1.45"),
		overflowWrap("anywhere"),
	)
	// The model's rephrasing is visibly secondary to the deterministic sentence
	// above it: same size, quieter colour, indented to the sentence's text column.
	rule(".budget-whyover-ai",
		margin("0"),
		paddingLeft("1.15rem"),
		color("var(--text-dim)"),
		overflowWrap("anywhere"),
	)
}
