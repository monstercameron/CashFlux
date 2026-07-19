// SPDX-License-Identifier: MIT

package styles

// registerAcctxn emits the styles for the 2026-07-19 Accounts + Transactions UX
// refinement pass: wider description/account columns on the ledger so they stop
// truncating, the ledger's row status-glyph legend, and a tighter "Upcoming"
// strip. Theme tokens only (works in light + dark); appended last in Register()
// so its equal-specificity overrides win over the earlier ledger rules.
func registerAcctxn() {
	// --- Wider ledger columns (stop truncating description + account) -------------
	// The ledger renders table-layout:fixed (.bento-ledger), so a column's width comes
	// from its HEADER cell. Give the Description header more of the row and a larger
	// floor, and let the Account column grow past the shared 11rem cap the secondary
	// columns share — long merchant names and account names were clipping mid-word.
	rule(".bento-ledger .txn-table th.row-desc-col",
		width("40%"),
	)
	rule(".txn-table td.row-desc-cell",
		minWidth("16rem"),
	)
	rule(".txn-table .td-acct",
		maxWidth("15rem"),
	)

	// --- Row status-glyph legend --------------------------------------------------
	// A single quiet line keying the ✓✓ / ✓ / • markers a row wears, so the status is
	// decoded in words rather than shape+color alone.
	rule(".txn-legend",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.15rem 0.85rem"),
		padding("0.35rem 0.1rem 0.1rem"),
		fontSize("0.72rem"),
		color("var(--text-dim)"),
	)
	rule(".txn-legend-label",
		fontWeight("600"),
		letterSpacing("0.03em"),
		textTransform("uppercase"),
		fontSize("0.66rem"),
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
