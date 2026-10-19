// SPDX-License-Identifier: MIT

package styles

// registerTC4W3 styles the receipt preview surface while its dialog behavior
// is migrated to the shared accessible overlay.
func registerTC4W3() {
	rule(".receipt-preview-overlay",
		position("fixed"),
		inset("0"),
		zIndex("var(--z-modal, 3000)"),
		display("grid"),
		placeItems("center"),
		padding("1rem"),
		background("color-mix(in srgb, var(--bg) 48%, transparent)"),
	)
}
