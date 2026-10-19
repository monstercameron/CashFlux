// SPDX-License-Identifier: MIT

package styles

// registerAcctxn emits the ledger status legend and the tighter "Upcoming"
// strip. Theme tokens only (works in light + dark); appended last in Register()
// so its equal-specificity overrides win over the earlier ledger rules.
func registerAcctxn() {
	// --- Row status-glyph legend --------------------------------------------------
	// A single quiet line keying the ✓✓ / ✓ / • markers a row wears, so the status is
	// decoded in words rather than shape+color alone.
	rule(".txn-legend",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.15rem 0.85rem"),
		padding("0.35rem 0.1rem 0.1rem"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
	)
	rule(".txn-legend-label",
		fontWeight("600"),
		letterSpacing("0.03em"),
		textTransform("uppercase"),
		fontSize("var(--type-11)"),
	)
	rule(".txn-legend-item",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		cursor("help"),
	)
	rule(".txn-legend-glyph",
		fontWeight("700"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
		minWidth("1.1rem"),
		textAlign("center"),
	)

	// --- Compressed "Upcoming" strip ----------------------------------------------
	// Trim the vertical padding so the pending-charges preview reads as a thin band
	// above the ledger instead of a tall block pushing the first real row down.
	rule(".txn-upcoming",
		padding("0.4rem 0.6rem"),
	)
	rule(".txn-upcoming-row",
		padding("0.12rem 0"),
	)
}
