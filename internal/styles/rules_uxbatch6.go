// SPDX-License-Identifier: MIT

package styles

// registerUxbatch6 is the single source for transaction-table column widths.
// Header and body cells share stable classes, so optional columns do not move
// width rules onto a different field.
func registerUxbatch6() {
	rule(".bento-ledger .txn-table th.td-select, .bento-ledger .txn-table td.td-select", width("40px"))
	rule(".bento-ledger .txn-table th.td-date, .bento-ledger .txn-table td.td-date", width("112px"))
	rule(".bento-ledger .txn-table th.td-cat, .bento-ledger .txn-table td.td-cat", width("170px"))
	rule(".bento-ledger .txn-table th.td-acct, .bento-ledger .txn-table td.td-acct", width("150px"))
	rule(".bento-ledger .txn-table th.td-user, .bento-ledger .txn-table td.td-user", width("56px"))
	rule(".bento-ledger .txn-table th.td-user", textAlign("center"))
	rule(".bento-ledger .txn-table th.td-amount, .bento-ledger .txn-table td.td-amount", width("112px"))
	rule(".bento-ledger .txn-table th.td-source, .bento-ledger .txn-table td.td-source", width("92px"))
	rule(".bento-ledger .txn-table th.td-actions, .bento-ledger .txn-table td.td-actions", width("76px"))
	rule(".bento-ledger .txn-table th.row-desc-col, .bento-ledger .txn-table td.row-desc-cell", width("auto"))
	rule(".bento-ledger .txn-table td.td-acct", overflow("hidden"), textOverflow("ellipsis"), whiteSpace("nowrap"))
	rule(".txn-transfer-category", color("var(--text-dim)"), whiteSpace("nowrap"))
	rule(".txn-review-pill", background("transparent"), color("var(--warn)"), border("1px solid var(--warn)"), borderRadius("999px"), padding("0.05rem 0.35rem"), fontSize("11px"), fontWeight("600"), whiteSpace("nowrap"))
	rule(".txn-member-avatar", display("inline-flex"), alignItems("center"), justifyContent("center"), width("22px"), height("22px"), borderRadius("50%"), background("var(--txn-member-color, var(--accent-dim))"), color("var(--text)"), fontSize("10px"), fontWeight("700"), lineHeight("1"))
}
