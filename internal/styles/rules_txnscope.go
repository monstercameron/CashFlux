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

	// --- The pending band, collapsed by default (C582) ----------------------------
	// It stops being one big navigating button: the band is a summary you read, and
	// the two things you might want to do with it — see the items, go to the
	// schedule — are named controls inside it. Collapsed it is a single line.
	rule(".txn-upcoming",
		cursor("default"),
		padding("0.4rem 0.85rem"),
		marginBottom("0.5rem"),
	)
	rule(".txn-upcoming:hover, .txn-upcoming:focus-visible",
		borderColor("var(--border)"),
	)
	rule(".txn-upcoming.is-collapsed .txn-upcoming-head",
		marginBottom("0"),
	)
	// The disclosure sits at the far end of the summary line, where the eye lands
	// after reading the fact — not before it.
	rule(".txn-upcoming-disc",
		marginLeft("auto"),
		flexShrink("0"),
		fontSize("var(--type-13)"),
	)
	rule(".txn-upcoming-link",
		marginTop("0.35rem"),
		fontSize("var(--type-13)"),
		alignSelf("flex-start"),
	)

	// --- Count line + legend share one row (C575 / C582) --------------------------
	// Both answer "how do I read the rows below", and every row of chrome above the
	// ledger is a row the user scrolls past to reach what they came for. The count
	// takes the left, the legend the right; they stack on narrow widths.
	rule(".txn-toolbar-foot",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.25rem 1rem"),
		padding("0.35rem 0.1rem 0.1rem"),
	)
	// The count is the sentence that gives every other number on the page its
	// denominator, so it is normal reading weight, not the dim caption the hidden
	// summary used to be.
	rule(".txn-countline",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.35rem"),
		fontSize("var(--type-13)"),
		color("var(--text)"),
	)
	rule(".txn-countline .txn-count-sep, .txn-countline .txn-count-net, .txn-countline .txn-count-lens",
		color("var(--text-dim)"),
	)
	rule(".txn-toolbar-foot .txn-legend",
		padding("0"),
	)

	// --- The primary Add, split (C573) --------------------------------------------
	// One control, two targets: press the label to add, press the caret to say what
	// kind first. They read as one object — a seam rather than a gap — so the caret
	// is unmistakably a modifier of the button beside it and not a fifth toolbar
	// action competing with it.
	rule(".btn-split",
		display("inline-flex"),
		alignItems("stretch"),
	)
	rule(".btn-split > .btn:first-child",
		prop("border-top-right-radius", "0"),
		prop("border-bottom-right-radius", "0"),
	)
	rule(".btn-split .btn-split-caret",
		prop("border-top-left-radius", "0"),
		prop("border-bottom-left-radius", "0"),
		borderLeft("1px solid rgba(0,0,0,0.22)"),
		paddingLeft("0.5rem"),
		paddingRight("0.5rem"),
	)

	// --- Row status badges say what they mean (C578) ------------------------------
	// Glyph and word sit on one line and never wrap or shrink: a status split across
	// two lines, or clipped to "Needs rev…", is worse than the bare dot it replaced.
	// The word is the quiet half — the glyph carries the scan, the word removes the
	// need to go and look up what the glyph meant.
	rule(".bento-ledger .txn-table th.td-status, .bento-ledger .txn-table td.td-status",
		width("104px"),
	)
	rule(".txn-table .td-status",
		fontSize("var(--type-13)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	// Colour is the second signal, never the only one: the word carries the meaning
	// and this only speeds the scan. Only the state that wants a person is tinted —
	// colouring every settled row would leave the row that needs attention with
	// nothing to set it apart.
	rule(".txn-table .td-status.txn-status-attention",
		color("var(--text)"),
		fontWeight("500"),
	)

	// --- Category provenance mark (C579) ------------------------------------------
	// Room for the mark. The Category column has been 90px since the uxbatch6 pass,
	// which already clipped ordinary names ("Guilty ple…", "Uncatego…") before
	// anything was added beside them; a provenance mark in a cell that narrow would
	// leave "Gro… auto". The 46px comes off the description's percentage share
	// (40% → 36%), which is the only column with slack: at 1440px it drops from
	// roughly 460px to 414px and still holds a full merchant line. Trading four
	// points of a column that was not truncating for a column that was is the right
	// direction.
	rule(".bento-ledger .txn-table th.row-desc-col",
		width("36%"),
	)
	rule(".bento-ledger .txn-table th.td-cat, .bento-ledger .txn-table td.td-cat",
		width("136px"),
	)
	// The category cell becomes "name + optional mark". The name truncates; the mark
	// never does, because a clipped provenance signal is worse than none — it would
	// read as part of the category.
	rule(".td-cat-inner",
		display("flex"),
		alignItems("baseline"),
		gap("0.3rem"),
		minWidth("0"),
	)
	rule(".td-cat-name",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	// Lower-case, dim and small: this marks the ORDINARY case (most imported rows
	// are unconfirmed), so it must not shout. It earns its place by being a word
	// rather than a colour, and by explaining itself on hover and to a screen reader.
	rule(".txn-auto-mark",
		fontSize("var(--type-11)"),
		lineHeight("1"),
		letterSpacing("0.02em"),
		color("var(--text-dim)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-pill)"),
		padding("0.1rem 0.3rem"),
		cursor("help"),
	)
}
