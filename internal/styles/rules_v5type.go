// SPDX-License-Identifier: MIT

package styles

// registerV5Type applies the audited Transactions type scale and flattens the
// shared primary action. It runs after the page and component rules so the scale
// is the final authority for transaction controls.
func registerV5Type() {
	rule(".bento-ledger .txn-table .td-cat, .bento-ledger .txn-table .td-acct",
		fontSize("var(--fs-body)"),
	)
	rule(`.bento-ledger .txn-table button, .bento > .w[data-widget="txn-toolbar"] button`,
		fontSize("var(--fs-body)"),
	)
	rule(".bento-ledger .txn-member-avatar, .bento-ledger .txn-table .td-user .cf-member-avatar, .bento-ledger .cf-member-avatar-stack .cf-member-avatar",
		fontSize("var(--fs-meta)"),
		fontWeight("600"),
	)
	rule(".bento-ledger .txn-catsuggest + .txn-cat-choose",
		fontSize("var(--fs-meta)"),
	)
	rule(".bento-ledger .txn-review-pill, .bento-ledger .txn-upcoming-badge",
		fontSize("var(--fs-meta)"),
	)
	rule(".btn-primary",
		background("var(--accent)"),
		borderColor("var(--accent)"),
		color("var(--primary-button-ink)"),
		boxShadow("none"),
	)
}
