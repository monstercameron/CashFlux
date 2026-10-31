// SPDX-License-Identifier: MIT

package styles

// registerAccountsS3 holds the Accounts rules for the sweep cards, the sweep-rules form and the
// reconcile panel. The shared Callout, KeyValue, Field and EntityRow components bring their own
// look; these rules only place them.
func registerAccountsS3() {
	// A sweep proposal: the live callout with its approve and dismiss buttons beneath it.
	rule(".acct-sweep-card",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".acct-sweep-card .callout-body p",
		margin("0"),
	)
	rule(".acct-sweep-actions",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
	)

	// A row that sits inside a wrapper element for its test id. The wrapper makes every row a
	// first child, so the divider that .row:first-child removes comes back for the second row on.
	rule(".acct-s3-item + .acct-s3-item > .row",
		borderTop("1px solid var(--border)"),
	)

	// Field labels in the sweep-rules form and the reconcile header read as quiet captions, the
	// look the stacked labels had before they moved to the shared Field.
	rule(".sweep-add .cf-field-label, .reconcile-header .cf-field-label",
		fontSize("var(--type-12)"),
		lineHeight("1.3"),
		color("var(--text-dim)"),
	)
	rule(".reconcile-header .cf-field",
		marginTop("0.4rem"),
	)

	// The reconcile worksheet: the closing line of the sum carries the weight.
	rule(".kv-value.acct-ws-total",
		fontWeight("600"),
	)

	// A history row's forced-reconciliation tag sits left of the statement balance.
	rule(".acct-recon-forced",
		marginRight("0.75rem"),
	)
}
