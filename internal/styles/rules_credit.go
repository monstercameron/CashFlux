// SPDX-License-Identifier: MIT

package styles

// registerCreditSurface emits the /credit bento-surface host rules. The tile
// internals reuse the shared hlt-* (factor tiles, formula block, variable
// chips) and credit-list/credit-item rules, so only the grid host is new.
func registerCreditSurface() {
	rule(".bento.bento-credit",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-credit > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// The not-a-FICO line carries chip weight (bordered, iconed) instead of
	// reading as the dimmest filler text in the hero.
	rule(".crd-disclaimer",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.45rem"),
		prop("padding", "0.3rem 0.7rem"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("font-size", "0.75rem"),
		prop("color", "var(--fg)"),
		prop("opacity", "0.85"),
		prop("max-width", "34rem"),
	)
	// The aggregate meter carries a tick at the 30% target so value-vs-target is
	// visual, not just prose.
	rule(".crd-meter-wrap",
		prop("position", "relative"),
	)
	rule(".crd-target-tick",
		prop("position", "absolute"),
		prop("left", "30%"),
		prop("top", "-3px"),
		prop("bottom", "-3px"),
		prop("width", "2px"),
		prop("background", "var(--fg)"),
		prop("opacity", "0.55"),
		prop("border-radius", "1px"),
		prop("pointer-events", "none"),
	)
}
