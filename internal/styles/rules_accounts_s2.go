// SPDX-License-Identifier: MIT

package styles

// registerAccountsS2 holds the layout rules for the Accounts page lists: the group editor, the
// institution directory, the document drawer, the services list and the recurring candidates. The
// rows themselves are the shared EntityRow, and the toggles and notices are the shared Disclosure
// and Callout; these rules only place them.
func registerAccountsS2() {
	// A list row that needs a test id sits inside a plain wrapper, because EntityRow takes no
	// attributes. The wrapper adds no box of its own, so the row lays out as a direct child of .rows,
	// and the separator the row draws between siblings is restored here: every .row would otherwise be
	// the first child of its own wrapper and lose its top rule.
	rule(".acct-entity",
		display("contents"),
	)
	rule(".rows > .acct-entity > .row",
		borderTop("1px solid var(--border)"),
	)
	rule(".rows > .acct-entity:first-child > .row",
		borderTop("0"),
	)

	// A callout that leads a form or a list keeps a gap before the content below it.
	rule(".acct-callout-gap",
		margin("0 0 0.75rem"),
	)

	// The documents drawer is a disclosure in the account panel's footer. Open, it takes the whole
	// footer line so the list and the attach form have room.
	rule(".acct-panel-footer > .acct-docs:has(.disclosure-body)",
		flexBasis("100%"),
	)
	rule(".acct-docs .disclosure-btn",
		fontSize("var(--type-14)"),
	)
	rule(".acct-docs .disclosure-body > div > .callout",
		margin("0.35rem 0"),
	)

	// The recurring candidates are a bordered list under the disclosure. The rows are EntityRow, so
	// the list pads them inside the border.
	rule(".acct-recur .disclosure-btn",
		fontSize("var(--type-14)"),
	)
	rule(".acct-recur .acct-recur-list .row",
		padding("0.7rem 0.9rem"),
		gap("0.9rem"),
	)
	rule(".acct-recur .disclosure-body > .callout",
		marginTop("0.35rem"),
	)
}
