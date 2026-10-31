// SPDX-License-Identifier: MIT

package styles

// registerRadioGroup styles the shared radio group: a quiet track of choices where the
// chosen one lifts in with a short settle, hover and press states respond at once, and
// every transition stops under reduced motion.
func registerRadioGroup() {
	keyframes("radioSettle",
		at("from", transform("scale(0.94)")),
		at("to", transform("scale(1)")),
	)
	rule(".radio-group",
		display("inline-flex"), alignItems("center"), gap("2px"), padding("3px"),
		border("1px solid var(--border)"), borderRadius("var(--radius-md)"), background("var(--bg-card)"),
	)
	rule(".radio-option",
		display("inline-flex"), alignItems("center"), justifyContent("center"),
		minHeight("2.25rem"), padding("0 0.9rem"),
		border("1px solid transparent"), borderRadius("calc(var(--radius-md) - 2px)"),
		background("transparent"), color("var(--text-dim)"),
		fontSize("var(--fs-body)"), fontWeight("500"), lineHeight("1"), cursor("pointer"),
		prop("transition", "background-color 180ms ease, color 180ms ease, border-color 180ms ease, box-shadow 180ms ease, transform 120ms ease"),
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
		background("color-mix(in srgb, var(--accent) 22%, var(--bg-card))"),
		borderColor("var(--accent)"), color("var(--text)"), fontWeight("600"),
		boxShadow("0 0 0 1px color-mix(in srgb, var(--accent) 35%, transparent)"),
		prop("animation", "radioSettle 220ms cubic-bezier(0.2, 0.7, 0.2, 1) both"),
	)
	rule(".radio-option.is-disabled",
		opacity("0.5"), cursor("not-allowed"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".radio-option, .radio-option.is-checked",
		prop("transition", "none"), prop("animation", "none"),
	)
}
