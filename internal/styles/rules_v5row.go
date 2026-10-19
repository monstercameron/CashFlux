// SPDX-License-Identifier: MIT

package styles

// registerV5Row keeps transaction controls reachable and raises notices above
// the fixed bulk-action bar while it is visible.
func registerV5Row() {
	rule(".bento-ledger .txn-ledger-row button",
		boxSizing("border-box"), minWidth("32px"), minHeight("32px"),
	)
	rule(".bento-ledger .txn-ledger-row input[type=checkbox]",
		boxSizing("border-box"), width("16px"), height("16px"),
		minWidth("16px"), minHeight("16px"), padding("0"), appearance("none"), webkitAppearance("none"),
		border("1px solid var(--control-border)"), borderRadius("3px"),
		background("var(--bg-elev)"), cursor("pointer"),
	)
	rule(".bento-ledger .txn-ledger-row input[type=checkbox]:checked",
		background("var(--accent)"), borderColor("var(--accent)"),
	)
	rule(".bento-ledger .txn-ledger-row input[type=checkbox]:checked::after",
		content(`""`), display("block"), width("8px"), height("4px"),
		margin("3px 0 0 3px"), borderLeft("2px solid #fff"),
		borderBottom("2px solid #fff"), transform("rotate(-45deg)"),
	)
	rule(".bento-ledger .txn-ledger-row .txn-row-menu > button[aria-haspopup], .bento-ledger .txn-ledger-row .txn-row-edit",
		width("32px"), height("32px"), minWidth("32px"), minHeight("32px"),
	)
	rule(".bento-ledger .txn-ledger-row .txn-row-menu [role=menuitem]",
		width("100%"), minWidth("160px"), minHeight("32px"),
	)
	rule("body:has(.bento-ledger.txn-has-selection) .toast",
		bottom("calc(60px + env(safe-area-inset-bottom, 0px))"),
	)
	rule(".toast-balance", display("block"), marginTop(".2rem"))
}
