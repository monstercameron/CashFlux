// SPDX-License-Identifier: MIT

package styles

// registerTxnUpcoming emits the ledger's pending-vs-posted strip: this month's
// still-unposted scheduled charges as ghost rows above the transactions table.
// The visual grammar is deliberate — dashed hairline, dimmed ink, an UPCOMING
// badge — so a schedule entry can never be misread as a posted transaction.
// Theme tokens only.
func registerTxnUpcoming() {
	rule(".txn-upcoming",
		display("block"),
		width("100%"),
		textAlign("left"),
		font("inherit"),
		color("inherit"),
		background("none"),
		border("1px dashed var(--border)"),
		borderRadius("var(--radius-lg)"),
		padding("0.6rem 0.85rem"),
		marginBottom("0.75rem"),
		cursor("pointer"),
	)
	rule(".txn-upcoming:hover, .txn-upcoming:focus-visible",
		borderColor("var(--accent)"),
	)
	rule(".txn-upcoming-head",
		display("flex"),
		alignItems("baseline"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginBottom("0.35rem"),
	)
	rule(".txn-upcoming-heading",
		fontWeight("600"),
		fontSize("var(--type-14)"),
		color("var(--text)"),
	)
	rule(".txn-upcoming-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	rule(".txn-upcoming-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		opacity("0.72"),
		fontSize("var(--type-14)"),
	)
	rule(".txn-upcoming-badge",
		flexShrink("0"),
		padding("0.02rem 0.4rem"),
		borderRadius("var(--radius-pill)"),
		border("1px dashed var(--border)"),
		color("var(--text-dim)"),
		fontSize("0.6rem"),
		fontWeight("600"),
		letterSpacing("0.08em"),
		textTransform("uppercase"),
	)
	rule(".txn-upcoming-amt",
		color("var(--text-dim)"),
	)
}
