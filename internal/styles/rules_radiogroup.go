// SPDX-License-Identifier: MIT

package styles

// registerRadioGroup styles the shared radio group. The options share equal columns, and
// a single indicator slides between them as the choice changes, so the selection travels
// from one radio to the next. Hover and press respond at once, and every motion stops
// under reduced motion.
func registerRadioGroup() {
	rule(".radio-group",
		position("relative"), display("inline-grid"), alignItems("stretch"), gap("2px"), padding("3px"),
		prop("grid-template-columns", "repeat(var(--radio-count, 1), minmax(0, 1fr))"),
		border("1px solid var(--border)"), borderRadius("var(--radius-lg)"), background("var(--bg-card)"),
	)
	rule(".radio-indicator",
		position("absolute"), top("3px"), bottom("3px"), left("3px"),
		width("calc((100% - 6px) / var(--radio-count, 1))"),
		borderRadius("calc(var(--radius-lg) - 3px)"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		border("1px solid color-mix(in srgb, var(--accent) 55%, var(--border))"),
		zIndex("0"), pointerEvents("none"),
		prop("transform", "translateX(calc(var(--radio-index, 0) * 100%))"),
		prop("transition", "transform 280ms cubic-bezier(0.2, 0.7, 0.2, 1), opacity 180ms ease"),
	)
	rule(".radio-group.is-empty .radio-indicator",
		opacity("0"),
	)
	rule(".radio-option",
		position("relative"), zIndex("1"),
		display("inline-flex"), alignItems("center"), justifyContent("center"),
		minHeight("2.25rem"), padding("0 0.9rem"),
		border("1px solid transparent"), borderRadius("calc(var(--radius-lg) - 3px)"),
		background("transparent"), color("var(--text-dim)"),
		fontSize("var(--fs-body)"), fontWeight("500"), lineHeight("1"), cursor("pointer"), whiteSpace("nowrap"),
		prop("transition", "color 220ms ease, background-color 180ms ease, border-color 180ms ease, transform 120ms ease"),
	)
	rule(".radio-option:hover:not(.is-checked):not(.is-disabled)",
		background("var(--bg-elev)"), borderColor("var(--border)"), color("var(--text)"),
	)
	rule(".radio-option:active:not(.is-disabled)",
		transform("scale(0.97)"),
	)
	rule(".radio-option:focus-visible",
		outline("2px solid var(--accent)"), outlineOffset("2px"),
	)
	rule(".radio-option.is-checked",
		// The weight stays the same in every state: a bolder label is wider, and a wider label
		// would resize the whole group each time the selection moves.
		color("var(--text)"),
	)
	rule(".radio-option.is-disabled",
		opacity("0.5"), cursor("not-allowed"),
	)
	// The sliding indicator is the selection itself, so it still moves under reduced motion.
	// Only the option hover and colour transitions stop.
	// Inside the filters panel the group fills its column, so the choices sit side by side.
	rule(".filter-fields .radio-group",
		display("grid"), width("100%"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".radio-option",
		prop("transition", "none"),
	)
}
