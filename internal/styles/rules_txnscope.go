// SPDX-License-Identifier: MIT

package styles

// registerTxnScope emits the styles for the transactions ledger's scope model
// (C574/C575/C578): the filter summary's lead-in, the "viewing as" member-lens
// chip, and the word a row shows beside its needs-review dot.
//
// Theme tokens only, so it works in light and dark. Registered late in install so
// its equal-specificity rules win over the earlier generic chip styles.
func registerTxnScope() {
	// --- Filter summary lead-in ---------------------------------------------------
	// The chip row is a sentence about the view ("Filtering by: Groceries, July"),
	// so the lead-in is quiet, uppercase and non-shrinking: a label, not a chip.
	rule(".filter-chips-lead",
		flexShrink("0"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)

	// --- The member-lens chip -----------------------------------------------------
	// It comes from the top bar, not the filter panel, and its ✕ clears the top bar.
	// An accent left edge marks it as a different KIND of chip so the difference is
	// visible before the click, not discovered after it.
	rule(".filter-chip.filter-chip-lens",
		borderColor("var(--accent)"),
		boxShadow("inset 2px 0 0 0 var(--accent)"),
	)
}
