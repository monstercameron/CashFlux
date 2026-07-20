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
	// The max bound only binds if the chain from the wrap down to the scrolling
	// body is unbroken. .flip-inner carries height:100% from the generated
	// defaults, which against an auto-height wrap resolves to auto — so the inner
	// sized to its content, the in-flow back face grew with it, and the wrap's
	// max-height clipped nothing (it has no overflow of its own). A tall panel
	// then rendered PAST the bottom of the viewport with its footer unreachable,
	// while .set-body — which is ready to scroll — was never given a height to
	// scroll within. Making the inner a flex item that may shrink (min-height:0)
	// restores the bound, and the body scrolls as the Height prop always claimed.
	rule(".flip-inner",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("flex", "1 1 auto"),
		prop("height", "auto"),
		prop("min-height", "0"),
	)
	rule(".flip-back",
		prop("position", "relative"),
		prop("flex", "1 1 auto"),
		prop("min-height", "0"),
	)
}
