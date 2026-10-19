// SPDX-License-Identifier: MIT

package styles

// registerTV8B applies the final transaction-row fixes from the visual review.
func registerTV8B() {
	rule(".bento-ledger .txn-table td.td-select",
		textAlign("center"),
	)
	rule(".bento-ledger .txn-table td.td-select input[type=checkbox]",
		margin("0 auto"),
	)
}
