// SPDX-License-Identifier: MIT

package styles

// registerTxcFieldsSurface emits the transaction-level comp-parity affordances:
// the excluded-from-reports row treatment + badge (TXC-1), the row note glyph
// (TXC-2), and the quick-filter preset chips (TXC-3). Theme tokens only.
func registerTxcFieldsSurface() {
	// TXC-1: an excluded row reads as "still real money, but out of the analysis" —
	// muted text with a struck amount, while its badge stays legible.
	rule(".row.txn-excluded .td-amount, .row.txn-excluded td:not(.td-amount)",
		color("var(--text-dim)"),
	)
	rule(".row.txn-excluded .td-amount",
		textDecoration("line-through"),
	)
	rule(".txn-excluded-badge",
		color("#d98c00"),
		borderColor("color-mix(in srgb, #d98c00 45%, transparent)"),
		background("color-mix(in srgb, #d98c00 12%, transparent)"),
	)

	// TXC-1: separate the "exclude from reports" control from the "Cleared
	// (reconciled)" checkbox above it with a hairline, so the two aren't confused.
	rule(".txn-exclude-field",
		marginTop("0.35rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 60%, transparent)"),
	)

	// TXC-2: the note glyph sits inline after the description, quiet until noticed.
	rule(".txn-note-glyph",
		display("inline-flex"),
		alignItems("center"),
		marginLeft("0.35rem"),
		color("var(--text-dim)"),
		verticalAlign("middle"),
	)

	// Follow-up chip: a small "open/total" pill after the description that links to the
	// filtered To-do list. Accented while any follow-up is open, muted once all are done.
	rule(".txn-followup-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		marginRight("0.45rem"),
		flexShrink("0"),
		padding("0.03rem 0.4rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.72rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		cursor("pointer"),
		verticalAlign("middle"),
		transition("border-color .12s ease, color .12s ease, background .12s ease"),
	)
	rule(".txn-followup-chip:hover",
		borderColor("var(--text-dim)"),
		color("var(--text)"),
	)
	rule(".txn-followup-chip.has-open",
		color("var(--accent)"),
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
	)
	rule(".txn-followup-chip.has-open:hover",
		background("color-mix(in srgb, var(--accent) 16%, transparent)"),
	)
	rule(".txn-followup-chip.all-done",
		opacity("0.65"),
	)

	// TXC-3: quick-filter preset chips above the ledger.
	rule(".txn-presets",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".txn-presets-label",
		fontSize("0.7rem"),
		fontWeight("600"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
		marginRight("0.15rem"),
	)
	rule(".txn-preset",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		padding("0.25rem 0.65rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		fontSize("0.8rem"),
		fontWeight("500"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("background .12s ease, border-color .12s ease, color .12s ease"),
	)
	rule(".txn-preset:hover",
		borderColor("var(--text-dim)"),
	)
	rule(".txn-preset.on",
		background("var(--accent)"),
		borderColor("var(--accent)"),
		color("#fff"),
	)
	rule(".txn-preset-count",
		fontSize("0.72rem"),
		opacity("0.75"),
		fontVariantNumeric("tabular-nums"),
	)
}
