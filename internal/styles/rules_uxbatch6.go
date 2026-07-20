// SPDX-License-Identifier: MIT

package styles

// registerUxbatch6 fixes the full-table ledger column balance in the wide band
// (content ~966-1042px, e.g. a 1260px viewport with the sidebar expanded).
//
// The base ledger sizes `.bento-ledger .txn-table` columns two ways that fight
// each other under table-layout:fixed:
//
//   - The Description header (rules_txcfields.go) asks for width:34%.
//   - Account / Category / Source get POSITIONAL nth-child pixel widths
//     (rules_gen.go: 184 / 150 / 118), and the actions column gets none.
//
// At ~1000px the fixed pixel columns (184+150+118 + select + date + amount) plus
// the 34% description over-subscribe the table. Fixed layout resolves that by
// shrinking the only flexible (percentage) column — Description — so it
// collapsed to ~150px and truncated "VENMO PAYMENT 1042778120" to "VEN…". A
// second bug: the nth-child widths are POSITIONAL, so when a column's visibility
// toggles (Amount hidden, or the User column appears) the indices shift and a
// fixed width lands on the wrong column.
//
// Fix, both robustly: size every secondary column by its stable td-* class (the
// header cells now carry the same class — see transactions_widget.go) instead of
// by position, and trim those columns hard so the Description's percentage share
// actually survives. Account/Category/Source/User read fine narrow (their
// content ellipsizes — the base `.bento-ledger .txn-table td` rule sets
// overflow/ellipsis/nowrap), and the everyday scan is date · payee · amount, so
// the description keeps priority. The actions column gets a fixed width so the ⋯
// cell stops competing for the leftover space. Select (col 1) stays on its base
// 40px nth-child; Date is pulled in a touch via nth-child(2) (always col 2).
//
// Registered LAST (install.go, before inject) so these class rules — which share
// the base rules' 0-3-1 specificity — win by source order. The 710-966
// condensed-card tier (rules_qpassEtxn.go) uses higher-specificity
// `.bento-ledger .txn-table tbody tr.row > td` selectors, so it still overrides
// these within its own band. Widths only; no theme tokens.
func registerUxbatch6() {
	// Date: always column 2 (Select is always column 1), so nth-child is safe and
	// stable here. Held at 116px — the base ledger width — so two-digit days
	// ("Jul 17, 2026", 12 chars) render in full; trimming this to 102 (an earlier
	// pass) clipped them to "Jul 17, 20…" in the wide band. The 14px this costs the
	// description over the trimmed value is reclaimed 1:1 from the secondary columns
	// below (acct/cat −4 each, source −6), so the description's percentage share is
	// unchanged and still clears its ≥16-char minimum ("VENMO PAYMENT 1042778120").
	rule(".bento-ledger .txn-table th:nth-child(2), .bento-ledger .txn-table td:nth-child(2)",
		width("116px"),
	)
	// Amount / running-balance: compact, tabular. Both carry td-amount.
	rule(".bento-ledger .txn-table th.td-amount, .bento-ledger .txn-table td.td-amount",
		width("106px"),
	)
	// Account / Category: the secondary scan columns — trimmed hard from 184/150
	// so the description (the primary read) keeps its share; content ellipsizes.
	// 90px: 4px each reclaimed for the date column, plus 6px each reclaimed for the
	// source column below. These two ellipsize by design, so the trim is invisible
	// on normal values and keeps the reclaim off the description's percentage share.
	rule(".bento-ledger .txn-table th.td-acct, .bento-ledger .txn-table td.td-acct",
		width("90px"),
	)
	rule(".bento-ledger .txn-table th.td-cat, .bento-ledger .txn-table td.td-cat",
		width("90px"),
	)
	// Source: 92px so the longest source value, "Recurring" (9 chars), renders in
	// full — 80px clipped it to "Recurri…". The extra 12px over that is reclaimed
	// 1:1 from acct/cat above (−6 each), so the description keeps its full share.
	rule(".bento-ledger .txn-table th.td-source, .bento-ledger .txn-table td.td-source",
		width("92px"),
	)
	rule(".bento-ledger .txn-table th.td-user, .bento-ledger .txn-table td.td-user",
		width("80px"),
	)
	// Actions: fixed so the ⋯ column no longer steals width from the description.
	rule(".bento-ledger .txn-table th.td-actions, .bento-ledger .txn-table td.td-actions",
		width("48px"),
	)
}
