// SPDX-License-Identifier: MIT

package styles

// registerVaultSurface emits the /artifacts file-vault design: the auto-row
// bento host and the storage meter that rides the hero. Hero/section/takeaway
// chrome reuses the shared rpt-*/debt-* rules so the page reads as a sibling
// of the Understand surfaces. Registered from Register().
func registerVaultSurface() {
	rule(".bento.bento-vault",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-vault > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	rule(".bento.bento-vault > .w:has(.add-menu:not(.hidden-menu))",
		prop("z-index", "30"),
	)
	// The hero's storage meter: wider than the row bars, capped for readability.
	rule(".vault-meter",
		prop("max-width", "30rem"),
		prop("margin-top", "0.5rem"),
	)
	// File rows read left-to-right: thumbnail beside the name/meta stack (the
	// base .row-main is a centered column, which stacked and centered them).
	rule(".bento-vault .row-main",
		prop("flex-direction", "row"),
		prop("align-items", "center"),
		prop("justify-content", "flex-start"),
		prop("gap", "0.6rem"),
		prop("text-align", "left"),
		prop("flex", "1"),
		prop("min-width", "0"),
	)
}
