// SPDX-License-Identifier: MIT

package styles

// registerR6F styles scheduled ledger rows with full-contrast text, a quiet
// dashed leading marker, and a compact status pill.
func registerR6F() {
	rule(".txn-scheduled-ghost",
		prop("color", "var(--text)"),
		prop("font-style", "normal"),
		prop("opacity", "1"),
		prop("border-left", "2px dashed var(--border)"),
	)
	rule(".txn-scheduled-ghost .td-acct",
		prop("min-width", "0"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-scheduled-pill",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("padding", "0.1rem 0.4rem"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("color", "var(--text)"),
		prop("font-size", "var(--type-11)"),
		prop("font-style", "normal"),
		prop("line-height", "1.3"),
		prop("white-space", "nowrap"),
	)
}
