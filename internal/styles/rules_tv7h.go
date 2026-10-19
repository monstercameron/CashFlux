// SPDX-License-Identifier: MIT

package styles

// registerTV7H fixes the pressed Transactions view label contrast.
func registerTV7H() {
	rule(`.txn-viewtoggle-btn[aria-pressed="true"]`,
		color("var(--primary-button-ink)"),
	)
}
