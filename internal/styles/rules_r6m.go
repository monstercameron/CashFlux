// SPDX-License-Identifier: MIT

package styles

// registerR6M keeps transaction row action labels accessible while simplifying
// the visible menu to a flat list with one divider before Delete.
func registerR6M() {
	rule(".txn-row-menu",
		display("contents"),
	)
	rule(".txn-row-menu .add-menu-section",
		display("none"),
	)
	rule(".txn-row-menu .add-menu-section:has(+ .add-item.danger)",
		display("block"),
		marginTop(".3rem"),
		paddingTop(".35rem"),
		borderTop("1px solid var(--border)"),
		fontSize("0"),
		lineHeight("0"),
	)
	rule(".txn-link-chooser",
		display("flex"),
		flexDirection("column"),
		gap(".5rem"),
	)
	rule(".txn-link-chooser .btn",
		justifyContent("flex-start"),
		minHeight("2.5rem"),
	)
}
