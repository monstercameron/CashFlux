// SPDX-License-Identifier: MIT

package styles

// registerSOL1 owns the ledger row's surface and explicit description action.
func registerSOL1() {
	rule(".txn-table tbody tr.txn-ledger-row", background("var(--txn-row-surface, var(--bg-card))"))
	// Cell surfaces cover the legacy shared .row:hover background, including its
	// light-theme important rule, without escalating specificity or importance.
	rule(".txn-ledger-row > td", background("var(--txn-row-surface, var(--bg-card))"))
	rule(".txn-ledger-row.selected", customProp("--txn-row-surface", "var(--accent-dim)"))
	rule(".txn-table .txn-description-edit",
		background("transparent"), border("0"), borderRadius("var(--radius-xs)"),
		padding("0"), margin("0"), color("inherit"), prop("font", "inherit"),
		textAlign("start"), cursor("pointer"), prop("text-decoration", "none"),
		prop("text-underline-offset", ".2em"),
	)
	ruleMedia("(hover: hover)", ".txn-ledger-row:not(.selected):hover",
		customProp("--txn-row-surface", "color-mix(in srgb, var(--hover) 45%, var(--bg-card))"))
	ruleMedia("(hover: hover)", ".txn-table .txn-description-edit:hover", prop("text-decoration", "underline"))
	rule(".txn-table .txn-description-edit:focus-visible",
		prop("text-decoration", "underline"), outline("2px solid var(--accent)"), outlineOffset("2px"))
}
