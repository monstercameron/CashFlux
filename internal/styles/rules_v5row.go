// SPDX-License-Identifier: MIT

package styles

// registerV5Row keeps transaction controls reachable and raises notices above
// the fixed bulk-action bar while it is visible.
func registerV5Row() {
	rule(".bento-ledger .txn-ledger-row button, .bento-ledger .txn-ledger-row input[type=checkbox]",
		boxSizing("border-box"), minWidth("32px"), minHeight("32px"),
	)
	rule(".bento-ledger .txn-ledger-row .txn-row-menu button, .bento-ledger .txn-ledger-row .txn-row-edit",
		width("32px"), height("32px"), minWidth("32px"), minHeight("32px"),
	)
	rule("body:has(.bento-ledger.txn-has-selection) .toast",
		bottom("calc(60px + env(safe-area-inset-bottom, 0px))"),
	)
	rule(".toast-balance", display("block"), marginTop(".2rem"))
}
