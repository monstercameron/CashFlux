// SPDX-License-Identifier: MIT

package styles

// registerFlipFit makes flip modals size to their CONTENT (UI/UX task #31):
// the wrap is a flex column whose height comes from the IN-FLOW back face —
// the decorative front face stays absolute for the 3D flip. Pairs with
// flippanel.go, where the Height prop became a max bound (height:auto +
// max-height inline), so sparse panels (Compare goals, Cover overages) hug
// their content while dense panels clamp and scroll exactly as before.
// Registered from Register() after the generated defaults.
func registerFlipFit() {
	rule(".flip-wrap",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".flip-back",
		prop("position", "relative"),
		prop("flex", "1 1 auto"),
		prop("min-height", "0"),
	)
}
