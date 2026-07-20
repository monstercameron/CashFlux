// SPDX-License-Identifier: MIT

package styles

import "fmt"

// registerTxnCondensedLedger adds the middle "condensed ledger" tier to the
// transactions table's responsive ladder (2026-07-19 v1.2.7 review, lane E #1).
//
// Before this, the ledger had exactly two regimes: a full multi-column table at
// content width >= contentGrid4 (966px), and — below that — a sparse stacked
// card where every field (date / amount / payee / account / category / source /
// member) sits on its own line (~270px per row). At expanded-sidebar desktop
// widths (~710-966 content px) that card wastes the pane and loses the columnar
// scan the ledger is for.
//
// This inserts a CONDENSED tier in the (contentGrid1, contentGrid4] band: a
// compact two-line row that keeps columnar scanability —
//
//	line 1:  [select]  date · payee (grows) · category · amount (right)
//	line 2:  account · member
//	actions: pinned to the row's right edge (absolute, never a text line)
//
// The genuinely-narrow full stacked card (rules_gen.go's ruleContentMax(
// contentGrid4) block rules) still governs panes below contentGrid1 (710px):
// this band's min-width gate excludes them, so those rules keep showing there.
//
// Layered via registerDtxPolish (registered last), and the per-column selectors
// carry higher specificity than the base block rules, so the condensed layout
// wins inside the band without editing rules_gen.go. Theme tokens only.
func registerTxnCondensedLedger() {
	const (
		descMin = "7rem"
		// The row's right padding clears the absolutely-positioned ⋯ actions so
		// they never overlap the meta line.
		rowPad = "0.45rem 3rem 0.45rem 0.6rem"
	)

	// The row becomes a two-line flex card. gen's ruleContentMax(contentGrid4)
	// already supplies the card border / radius / margin-bottom and hides the
	// thead; here we only override the interior layout (display, padding, gaps).
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		prop("column-gap", "0.75rem"),
		rowGap("0.1rem"),
		position("relative"),
		padding(rowPad),
	)

	// Base cell reset for the card: block flex-children with no border/padding of
	// their own (higher specificity than gen's `.txn-table tbody td`).
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td",
		display("block"),
		width("auto"),
		minWidth("0"),
		padding("0"),
		border("0"),
	)

	// Line 1 — select (col 1, no class), date (col 2, no class), payee, category,
	// amount. Payee grows (flex 1 1 0) so it fills line one, which forces the
	// account/member cells to wrap onto line two deterministically.
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td:first-child",
		order("0"),
		flex("0 0 auto"),
		alignSelf("center"),
	)
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td:nth-child(2)",
		order("1"),
		flex("0 0 auto"),
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.row-desc-cell",
		order("2"),
		flex("1 1 0"),
		minWidth(descMin),
	)
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-cat",
		order("3"),
		flex("0 1 auto"),
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		maxWidth("12rem"),
	)
	// Amount sits at the right end of line one; gen turned it left-aligned for the
	// stacked card, so re-assert the right alignment for the columnar read.
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-amount",
		order("4"),
		flex("0 0 auto"),
		textAlign("right"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)

	// Line 2 — the secondary meta strip: account then member, quiet and
	// ellipsized so they never widen the card.
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-acct",
		order("5"),
		flex("0 1 auto"),
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		maxWidth("45%"),
	)
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-user",
		order("6"),
		flex("0 1 auto"),
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		maxWidth("40%"),
	)
	// Source is tertiary in a condensed row (the edit modal still carries it);
	// dropping it is what keeps the card to two lines.
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-source",
		display("none"),
	)

	// The ⋯ actions pin to the right-center of the card instead of consuming a
	// line. Centered via inset + flex (NOT translateY — a transform here would
	// make this cell the containing block for the menu's fixed-position sheet).
	ruleContentBand(contentGrid1, contentGrid4, ".txn-table tbody tr.row > td.td-actions",
		order("7"),
		position("absolute"),
		right("0.5rem"),
		top("0"),
		bottom("0"),
		display("flex"),
		alignItems("center"),
	)
}

// ruleContentBand emits decls for selector whenever the content pane's width is
// in the half-open band (minContent, maxContent] — i.e. wider than minContent
// and at most maxContent — in either rail state. It is the two-sided companion
// to ruleContentMax / ruleContentMin (breakpoints.go): because the band's lower
// bound differs per rail state, BOTH emissions carry a zero-specificity
// :where(rail-state) prefix (there is no bare-selector shortcut as ruleContentMax
// has), so each keeps exactly the specificity of the bare selector.
func ruleContentBand(minContent, maxContent int, selector string, decls ...decl) {
	// Collapsed rail: pane = viewport - 58.
	ruleMedia(
		fmt.Sprintf("(min-width: %dpx) and (max-width: %dpx)", minContent+railCollapsedPx+1, maxContent+railCollapsedPx),
		prefixEachSelector(":where(html."+railStateClass+") ", selector), decls...)
	// Expanded rail (class absent, the conservative default): pane = viewport - 240.
	ruleMedia(
		fmt.Sprintf("(min-width: %dpx) and (max-width: %dpx)", minContent+railExpandedPx+1, maxContent+railExpandedPx),
		prefixEachSelector(":where(html:not(."+railStateClass+")) ", selector), decls...)
}
