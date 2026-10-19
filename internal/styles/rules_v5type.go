// SPDX-License-Identifier: MIT

package styles

// registerV5Type applies the audited Transactions type scale and flattens the
// shared primary action. It runs after the page and component rules so the scale
// is the final authority for transaction controls.
func registerV5Type() {
	// The audit covers the transaction dialogs as well as the ledger. Keep these
	// overrides together so dialogs inherit the same four steps and three weights.
	rule(".flip-back .set-h h3",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
	)
	rule(".rvs, .rvs button, .rvs input, .rvs select, .rvs textarea, .rvs summary, .rvs p, .rvs span, .rvs [class*='rvw-'], .txn-cal, .txn-cal *, .txn-edit .mtrend-spark-xaxis, .txn-edit .mtrend-spark-yaxis, .txn-edit .mtrend-spark-meta",
		fontFamily("Inter, sans-serif"),
	)
	rule(".rvs button, .txn-form-add label, .txn-form-edit label",
		fontWeight("500"),
	)
	rule(".rvs [class*='rvs-'], .rvs [class*='rvw-']",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
		fontStyle("normal"),
		textTransform("none"),
	)
	rule(".rvs .rvs-intro-title",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
	)
	rule(".rvs .rvs-big",
		fontSize("var(--fs-title)"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".txn-form-add .labeled-field > div:first-child, .txn-form-edit .labeled-field > div:first-child, .txn-form-add .qa-sentence > label",
		fontWeight("500"),
	)
	rule(".rvs .rvs-tier-count, .rvs .rvs-foot-sub, .rvs .rvs-note, .rvs .rvs-kbd, .rvs .rvw-rawpayee, .txn-edit .mtrend-spark-xaxis, .txn-edit .mtrend-spark-yaxis, .txn-edit .mtrend-spark-meta, .txn-cal .txn-cal-ghost, .txn-cal .txn-cal-week-count",
		fontSize("var(--fs-meta)"),
		fontWeight("400"),
		fontStyle("normal"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".txn-cal .txn-cal-ghost-transfer, .txn-cal .txn-cal-transfer",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		fontStyle("normal"),
	)
	rule(".txn-cal .txn-cal-moved",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
		fontStyle("normal"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".txn-edit .mtrend-spark-xaxis, .txn-edit .mtrend-spark-yaxis",
		color("var(--muted)"),
	)
	rule(".empty[data-testid='txn-nomatch'] .btn-link",
		fontSize("var(--fs-body)"),
		fontWeight("500"),
		fontStyle("normal"),
	)
	rule(".rvs .rvs-tier-count, .rvs .rvs-intro-count",
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs .rvs-kbd",
		textTransform("none"),
		fontStyle("normal"),
	)
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
	rule(".bento-ledger .txn-table .td-user, .notify-btn .notify-badge",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		fontVariantNumeric("tabular-nums"),
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
