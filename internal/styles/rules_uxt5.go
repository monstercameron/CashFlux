// SPDX-License-Identifier: MIT

package styles

// registerUXT5 keeps transaction metadata quiet, anchored, and brief.
func registerUXT5() {
	rule(".txn-row-info",
		width("24px"), height("24px"), flex("0 0 24px"),
		padding("0"), border("0"), borderRadius("50%"),
		background("transparent"), color("var(--text-dim)"),
		fontSize("16px"), lineHeight("1"), cursor("pointer"),
	)
	rule(".txn-row-info-wrap", position("relative"), flex("0 0 24px"), width("24px"), height("24px"))
	rule(".txn-table td.row-desc-cell .row-desc-inner",
		flexDirection("column"), alignItems("stretch"), gap("0"),
	)
	rule(".txn-table td.row-desc-cell .txn-desc-mainline",
		display("flex"), alignItems("center"), gap("0.4rem"), minWidth("0"), width("100%"),
	)
	rule(".txn-table td.row-desc-cell .txn-desc-tags",
		alignSelf("flex-start"), maxWidth("100%"), width("100%"), height("19px"), flex("0 0 19px"),
	)
	rule(".txn-row-info:hover, .txn-row-info:focus-visible",
		background("var(--hover)"), color("var(--text)"),
	)
	rule(".txn-row-info-popover",
		position("absolute"), right("0"), top("calc(100% + 4px)"), zIndex("40"),
		display("flex"), flexDirection("column"), gap(".35rem"),
		width("max-content"), padding(".5rem"), border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"), background("var(--bg-elev)"),
		boxShadow("0 8px 24px rgba(0,0,0,.22)"),
	)
	rule(".txn-transfer-category",
		display("block"), maxWidth("100%"), overflow("hidden"),
		textOverflow("ellipsis"), whiteSpace("nowrap"),
	)
	rule(".txn-count-upcoming", position("relative"))
	rule(".txn-upcoming-popover",
		position("absolute"), top("calc(100% + 6px)"), left("0"), zIndex("30"),
		width("max-content"), maxWidth("min(30rem, calc(100vw - 2rem))"),
		padding(".65rem"), border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"), background("var(--bg-elev)"),
		boxShadow("0 8px 24px rgba(0,0,0,.22)"),
	)
	rule(".txn-row-flash", animation("txn-row-flash 150ms ease-out"))
	rawBlock("@keyframes txn-row-flash{0%{background-color:var(--accent-soft)}100%{background-color:transparent}}")
	ruleMedia("(prefers-reduced-motion: reduce)", ".txn-row-flash", animation("none"))
}
