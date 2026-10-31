// SPDX-License-Identifier: MIT

package styles

// registerMultiPicker styles the shared multi-select. The trigger is a standard field and
// the sheet opens beside it with a check mark on each chosen option. The picker sets only
// the sheet's placement inline, so every colour, radius, size and shadow here comes from
// the shared tokens and every page gets the same control.
func registerMultiPicker() {
	rule(".cf-multi-trigger",
		position("relative"), display("flex"), alignItems("center"), justifyContent("space-between"),
		gap("0.4rem"), width("100%"), minWidth("0"), minHeight("calc(var(--control-h) + 8px)"), boxSizing("border-box"),
		padding("0.4rem 2rem 0.4rem 0.6rem"),
		background("var(--bg-elev)"), border("1px solid var(--border)"),
		borderRadius("var(--radius-md)"), color("var(--text)"), font("inherit"),
		fontSize("var(--fs-body)"), textAlign("left"), whiteSpace("nowrap"), overflow("hidden"),
		textOverflow("ellipsis"), cursor("pointer"),
		prop("transition", "border-color .12s ease, box-shadow .12s ease"),
	)
	rule(".cf-multi-trigger:focus-visible, .cf-multi-trigger[aria-expanded=\"true\"]",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
	)
	// The chevron is drawn here, not taken from a native select, so the field reads the same
	// wherever it sits. It turns to point up while the sheet is open.
	rule(".cf-multi-trigger::after",
		prop("content", `""`), prop("position", "absolute"), prop("right", "0.9rem"), prop("top", "45%"),
		prop("width", "0.42rem"), prop("height", "0.42rem"),
		prop("border-right", "1.5px solid currentColor"), prop("border-bottom", "1.5px solid currentColor"),
		prop("transform", "translateY(-70%) rotate(45deg)"), prop("pointer-events", "none"),
		prop("transition", "transform .15s ease"),
	)
	rule(".cf-multi-trigger[aria-expanded=\"true\"]::after",
		prop("transform", "translateY(-30%) rotate(-135deg)"),
	)

	rule(".cf-multi-surface",
		boxSizing("border-box"), padding("0.35rem"),
		background("var(--bg-elev)"), border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"), prop("box-shadow", "var(--shadow-2)"),
		overflowY("auto"), prop("scrollbar-width", "thin"),
	)
	rule(".cf-multi-title",
		padding("0.4rem 0.6rem 0.45rem"), fontSize("var(--fs-meta)"), fontWeight("600"), color("var(--text-dim)"),
	)
	rule(".cf-multi-list",
		display("flex"), flexDirection("column"), gap("2px"),
	)
	rule(".cf-multi-option",
		position("relative"), display("flex"), alignItems("center"), justifyContent("space-between"),
		gap("0.5rem"), width("100%"), boxSizing("border-box"), minHeight("var(--control-h)"),
		padding("0.45rem 1.9rem 0.45rem 0.6rem"),
		background("transparent"), border("1px solid transparent"), borderRadius("var(--radius-md)"),
		color("var(--text)"), font("inherit"), fontSize("var(--fs-body)"), textAlign("left"), cursor("pointer"),
		prop("transition", "background-color .12s ease, border-color .12s ease"),
	)
	rule(".cf-multi-option:hover",
		background("color-mix(in srgb, var(--text) 6%, transparent)"),
	)
	rule(".cf-multi-option[aria-selected=\"true\"]",
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
	)
	rule(".cf-multi-option[aria-selected=\"true\"]::after",
		prop("content", `""`), prop("position", "absolute"), prop("right", "0.85rem"), prop("top", "45%"),
		prop("width", "0.32rem"), prop("height", "0.62rem"),
		prop("border-right", "2px solid var(--accent)"), prop("border-bottom", "2px solid var(--accent)"),
		prop("transform", "rotate(45deg)"), prop("pointer-events", "none"),
	)
	rule(".cf-multi-option:focus-visible",
		prop("outline", "2px solid var(--accent)"), prop("outline-offset", "-2px"),
	)
	rule(".cf-multi-option[aria-disabled=\"true\"]",
		prop("opacity", "0.5"), cursor("not-allowed"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".cf-multi-trigger, .cf-multi-trigger::after, .cf-multi-option",
		prop("transition", "none"),
	)
}
