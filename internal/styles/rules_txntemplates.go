// SPDX-License-Identifier: MIT

package styles

// registerTxnTemplatesSurface emits the chrome for transaction quick-templates
// ("favourites") mounted inside the quick-add flip panel: the horizontal picker
// strip of template chips, the per-chip delete affordance, the "Save as template"
// action, and the empty-state prompt. The coordinator wires this into Register().
//
// All colours use the theme tokens (var(--text)/(--border)/(--bg-card)/
// (--bg-elev)/--accent); never var(--fg)/(--line)/(--dim)/(--faint) (those are
// undefined and render dark in both themes). Every selector is prefixed .txt-.
func registerTxnTemplatesSurface() {
	// --- The picker strip: a wrapping row of template chips ----------------------
	rule(".txt-picker",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
		margin("0 0 0.25rem"),
	)

	// The empty-state prompt shown when there are no templates yet — quiet, small.
	rule(".txt-empty",
		margin("0"),
		fontSize("0.76rem"),
		lineHeight("1.35"),
		color("var(--text-dim)"),
	)

	// --- One template chip: a pill with the payee/name + its amount --------------
	rule(".txt-chip",
		position("relative"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		maxWidth("100%"),
		padding("0.3rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("color-mix(in srgb, var(--bg-elev) 55%, transparent)"),
		color("var(--text)"),
		fontSize("0.8rem"),
		fontWeight("500"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("border-color 0.14s ease, background 0.14s ease, color 0.14s ease"),
	)
	rule(".txt-chip:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
		color("var(--text)"),
	)
	rule(".txt-chip:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	// The chip's leading label (template name / payee), clipped so a long name never
	// blows out the strip.
	rule(".txt-chip-name",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		minWidth("0"),
	)
	// The trailing amount, quieter and tabular so figures line up chip-to-chip.
	rule(".txt-chip-amt",
		color("var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		fontWeight("600"),
	)

	// --- Per-chip delete affordance (small ✕, revealed on hover) -----------------
	rule(".txt-chip-del",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.05rem"),
		height("1.05rem"),
		padding("0"),
		borderRadius("999px"),
		border("none"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		lineHeight("1"),
		cursor("pointer"),
		opacity("0"),
		transition("opacity 0.12s ease, background 0.12s ease, color 0.12s ease"),
	)
	rule(".txt-chip:hover .txt-chip-del, .txt-chip:focus-within .txt-chip-del",
		opacity("1"),
	)
	rule(".txt-chip-del:hover",
		background("color-mix(in srgb, var(--danger) 15%, transparent)"),
		color("var(--danger)"),
	)

	// --- The "Save as template" action, sat near the submit controls ------------
	rule(".txt-save-btn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		alignSelf("flex-start"),
		marginTop("0.35rem"),
		padding("0.3rem 0.75rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.78rem"),
		fontWeight("500"),
		cursor("pointer"),
		transition("border-color 0.14s ease, color 0.14s ease, background 0.14s ease"),
	)
	rule(".txt-save-btn:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".txt-save-btn:disabled",
		opacity("0.5"),
		cursor("not-allowed"),
	)

	// --- The templates zone: picker chips + "Save as template", grouped at the top of
	// the quick-add form and set off from the fields below by a hairline. -----------
	rule(".txt-zone",
		display("flex"),
		alignItems("flex-start"),
		justifyContent("space-between"),
		gap("0.6rem"),
		flexWrap("wrap"),
		paddingBottom("0.7rem"),
		marginBottom("0.2rem"),
		borderBottom("1px solid color-mix(in srgb, var(--border) 60%, transparent)"),
	)
	rule(".txt-zone .txt-picker",
		flex("1 1 auto"),
		minWidth("0"),
		margin("0"),
	)
	rule(".txt-zone .txt-save-btn",
		flexShrink("0"),
	)
}
