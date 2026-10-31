// SPDX-License-Identifier: MIT

package styles

// registerUIComponents styles the shared ui components Disclosure, Callout and KeyValue, and the
// trailing value slot of EntityRow. Colour comes from the theme tokens, so light and dark both
// follow. Registered from Register().
func registerUIComponents() {
	// Disclosure: a full-width real button that shows and hides a body region.
	rule(".disclosure-btn",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
		width("100%"),
		padding("0.4rem 0"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		fontSize("inherit"),
		textAlign("left"),
		cursor("pointer"),
	)
	rule(".disclosure-btn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".disclosure-summary",
		minWidth("0"),
		flex("1"),
	)
	rule(".disclosure-chevron",
		flexShrink("0"),
		color("var(--text-dim)"),
		transition("transform 0.2s ease"),
	)
	rule(".disclosure-btn[aria-expanded=\"true\"] .disclosure-chevron",
		transform("rotate(180deg)"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".disclosure-chevron",
		transition("none"),
	)

	// Callout: an inline notice with a toned edge. The base is the info tone.
	rule(".callout",
		display("flex"),
		alignItems("flex-start"),
		justifyContent("space-between"),
		gap("0.75rem"),
		padding("0.6rem 0.8rem"),
		borderRadius("var(--radius-md)"),
		border("1px solid var(--border)"),
		borderLeft("3px solid var(--text-faint)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		fontSize("var(--type-14)"),
	)
	rule(".callout-text",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("0"),
		flex("1"),
	)
	rule(".callout-title",
		color("var(--text)"),
		fontWeight("600"),
	)
	rule(".callout-action",
		flexShrink("0"),
	)
	rule(".callout-warn",
		borderLeftColor("var(--warn, #d97706)"),
		background("color-mix(in srgb, var(--warn, #d97706) 8%, var(--bg-elev))"),
	)
	rule(".callout-danger",
		borderLeftColor("var(--danger)"),
		background("color-mix(in srgb, var(--danger) 8%, var(--bg-elev))"),
	)
	rule(".callout-ok",
		borderLeftColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 8%, var(--bg-elev))"),
	)
	ruleMedia("(max-width: 760px)", ".callout",
		flexWrap("wrap"),
	)

	// KeyValue: a list of facts. Each row puts the label on the left and the value on the right.
	rule(".kv",
		display("flex"),
		flexDirection("column"),
		margin("0"),
	)
	rule(".kv-row",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("1rem"),
		padding("0.35rem 0"),
		borderTop("1px solid var(--border)"),
	)
	rule(".kv-row:first-child",
		borderTop("0"),
	)
	rule(".kv-label",
		margin("0"),
		minWidth("0"),
	)
	rule(".kv-value",
		margin("0"),
		minWidth("0"),
		color("var(--text)"),
		textAlign("right"),
		overflowWrap("anywhere"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".kv-value.pos",
		color("var(--money-positive)"),
	)
	rule(".kv-value.neg",
		color("var(--money-negative)"),
	)
	rule(".kv-value.dim",
		color("var(--text-dim)"),
	)

	// EntityRow trailing value: stays on one line beside the text block.
	rule(".row-value",
		flexShrink("0"),
		textAlign("right"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)
}
