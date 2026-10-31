// SPDX-License-Identifier: MIT

package styles

// registerAccountsS4 holds the rules for the Accounts summary, the list notices and the
// transfer form (accounts_tiles.go). The shared Callout and Field components bring their own
// look; these rules only place them on this page.
func registerAccountsS4() {
	// The summary tile stacks its notices under the headline with a small gap.
	rule(".acct-summary-body > .callout",
		margin("0.6rem 0 0"),
	)
	// The collapsed-stale notice that leads the account list sits above the rows.
	rule(".callout[data-testid=\"acct-stale-summary\"]",
		margin("0 0 0.6rem"),
	)

	// Transfer form: the shared Field label, hint and error read like the form's other captions.
	rule(".acct-xfer-form .cf-field",
		width("100%"),
		minWidth("0"),
	)
	rule(".acct-xfer-form .cf-field-label, .acct-xfer-form .cf-field-hint",
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
		lineHeight("1.3"),
	)
	rule(".acct-xfer-form .cf-field-error",
		fontSize("var(--type-12)"),
		color("var(--danger)"),
	)
	// The before/after balance lines stack inside their notice.
	rule(".acct-xfer-preview",
		display("grid"),
		gap("0.15rem"),
	)
}
