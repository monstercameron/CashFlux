// SPDX-License-Identifier: MIT

package styles

// registerDesignPolish is the 2026-07-19 design-system refinement pass (frontend
// design review): app-wide rules that make the interface calmer and more precise
// without a visual reinvention. Registered LAST in install.go so these win the
// cascade over the generated base rules. Theme tokens only; light + dark both track.
//
// Pass 1 — precision typography: every figure in a financial workspace should be
// tabular so amounts, percentages, day counts, and dates align on the digit column
// instead of jittering. Many components already opt in; setting it at the app root
// makes it universal (tabular-nums only changes digit advance width — it never
// affects letters or layout), so no amount is left proportional.
func registerDesignPolish() {
	rule("#app",
		fontVariantNumeric("tabular-nums"),
	)
}
